package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	berth "github.com/tech-arch1tect/berth-go-api-client"
	"github.com/tech-arch1tect/terraform-provider-berth/internal/client"
)

var _ resource.Resource = &S3BucketResource{}

func NewS3BucketResource() resource.Resource {
	return &S3BucketResource{}
}

type S3BucketResource struct {
	client *client.Client
}

type S3BucketResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Label           types.String `tfsdk:"label"`
	Endpoint        types.String `tfsdk:"endpoint"`
	Region          types.String `tfsdk:"region"`
	BucketName      types.String `tfsdk:"bucket_name"`
	AccessKeyID     types.String `tfsdk:"access_key_id"`
	SecretAccessKey types.String `tfsdk:"secret_access_key"`
}

func (r *S3BucketResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket"
}

func (r *S3BucketResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an S3 bucket credential that Berth uses for stack backups. Requires an admin token with the admin.servers.write scope. Import is unsupported because the secret access key cannot be read back from the server",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "S3 bucket ID",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"label": schema.StringAttribute{
				Description: "Display label of the bucket",
				Required:    true,
			},
			"endpoint": schema.StringAttribute{
				Description: "S3 endpoint the bucket lives on (an http or https URL with a host)",
				Required:    true,
			},
			"region": schema.StringAttribute{
				Description: "Region of the bucket. The server stores a blank or explicitly empty region as us-east-1, so it reads back as us-east-1 and the adopted value is written back to state",
				Optional:    true,
				Computed:    true,
			},
			"bucket_name": schema.StringAttribute{
				Description: "Name of the bucket at the endpoint (lowercase letters, digits, dots and hyphens)",
				Required:    true,
			},
			"access_key_id": schema.StringAttribute{
				Description: "Access key ID used to reach the bucket",
				Required:    true,
				Sensitive:   true,
			},
			"secret_access_key": schema.StringAttribute{
				Description: "Secret access key for the bucket. Write-only: the server never returns it, so it lives in Terraform state and is re-sent on every update. The server does not check the credentials against the endpoint, so a wrong secret only surfaces when a backup fails",
				Required:    true,
				Sensitive:   true,
			},
		},
	}
}

func (r *S3BucketResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *S3BucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data S3BucketResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.createS3Bucket(&data); err != nil {
		resp.Diagnostics.AddError("Failed to create S3 bucket", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *S3BucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data S3BucketResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	gone, err := r.refreshS3Bucket(&data)
	switch {
	case gone:
		resp.State.RemoveResource(ctx)
		return
	case err != nil:
		resp.Diagnostics.AddError("Failed to read S3 bucket", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *S3BucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data S3BucketResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.updateS3Bucket(&data); err != nil {
		resp.Diagnostics.AddError("Failed to update S3 bucket", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *S3BucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data S3BucketResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.deleteS3Bucket(&data); err != nil {
		resp.Diagnostics.AddError("Failed to delete S3 bucket", err.Error())
		return
	}
}

func (r *S3BucketResource) createS3Bucket(data *S3BucketResourceModel) error {
	if s3BucketSecret(data) == "" {
		return errors.New("secret_access_key is required to create the bucket credential")
	}

	bucket, err := r.client.CreateS3Bucket(
		data.Label.ValueString(),
		data.Endpoint.ValueString(),
		s3BucketRegion(data),
		data.BucketName.ValueString(),
		data.AccessKeyID.ValueString(),
		data.SecretAccessKey.ValueString(),
	)
	if err != nil {
		return err
	}
	if bucket.Id == 0 {
		return errors.New("the server returned no bucket ID")
	}

	data.ID = types.StringValue(strconv.FormatUint(uint64(bucket.Id), 10))
	applyS3BucketResponse(data, *bucket)
	return nil
}

func (r *S3BucketResource) refreshS3Bucket(data *S3BucketResourceModel) (bool, error) {
	id, err := s3BucketID(data)
	if err != nil {
		return false, err
	}

	bucket, err := r.client.GetS3Bucket(id)
	switch classifyRead(err) {
	case readGone:
		return true, nil
	case readFailed:
		return false, err
	}

	applyS3BucketResponse(data, *bucket)
	return false, nil
}

func (r *S3BucketResource) updateS3Bucket(data *S3BucketResourceModel) error {
	if s3BucketSecret(data) == "" {
		return errors.New("secret_access_key must stay set so the stored credential keeps working")
	}

	id, err := s3BucketID(data)
	if err != nil {
		return err
	}

	bucket, err := r.client.UpdateS3Bucket(
		id,
		data.Label.ValueString(),
		data.Endpoint.ValueString(),
		s3BucketRegion(data),
		data.BucketName.ValueString(),
		data.AccessKeyID.ValueString(),
		data.SecretAccessKey.ValueString(),
	)
	if err != nil {
		return err
	}

	applyS3BucketResponse(data, *bucket)
	return nil
}

func (r *S3BucketResource) deleteS3Bucket(data *S3BucketResourceModel) error {
	id, err := s3BucketID(data)
	if err != nil {
		return err
	}

	err = r.client.DeleteS3Bucket(id)
	if errors.Is(err, client.ErrNotFound) {
		return nil
	}
	return err
}

func s3BucketID(data *S3BucketResourceModel) (uint, error) {
	id, err := strconv.ParseUint(data.ID.ValueString(), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid bucket ID: %w", err)
	}
	return uint(id), nil
}

func s3BucketRegion(data *S3BucketResourceModel) string {
	if data.Region.IsNull() || data.Region.IsUnknown() {
		return ""
	}
	return data.Region.ValueString()
}

func s3BucketSecret(data *S3BucketResourceModel) string {
	if data.SecretAccessKey.IsNull() || data.SecretAccessKey.IsUnknown() {
		return ""
	}
	return data.SecretAccessKey.ValueString()
}

func applyS3BucketResponse(data *S3BucketResourceModel, bucket berth.BucketResponse) {
	data.Label = types.StringValue(bucket.Label)
	data.Endpoint = types.StringValue(bucket.Endpoint)
	data.Region = types.StringValue(bucket.Region)
	data.BucketName = types.StringValue(bucket.BucketName)
	data.AccessKeyID = types.StringValue(bucket.AccessKeyId)
}
