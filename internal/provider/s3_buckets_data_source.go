package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tech-arch1tect/berth-go-api-client"
	"github.com/tech-arch1tect/terraform-provider-berth/internal/client"
)

var _ datasource.DataSource = &S3BucketsDataSource{}

func NewS3BucketsDataSource() datasource.DataSource {
	return &S3BucketsDataSource{}
}

type S3BucketsDataSource struct {
	client *client.Client
}

type S3BucketsDataSourceModel struct {
	Buckets []S3BucketDataSourceModel `tfsdk:"buckets"`
}

type S3BucketDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Label       types.String `tfsdk:"label"`
	Endpoint    types.String `tfsdk:"endpoint"`
	Region      types.String `tfsdk:"region"`
	BucketName  types.String `tfsdk:"bucket_name"`
	AccessKeyID types.String `tfsdk:"access_key_id"`
}

func (d *S3BucketsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_buckets"
}

func (d *S3BucketsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Every S3 bucket configured for Berth stack backups. The secret access key is never returned by the API, only its access key ID. Requires an admin token with the admin.servers.read scope",
		Attributes: map[string]schema.Attribute{
			"buckets": schema.ListNestedAttribute{
				Description: "All S3 buckets, ordered by ID as the server returns them",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "S3 bucket ID",
							Computed:    true,
						},
						"label": schema.StringAttribute{
							Description: "Display label of the bucket",
							Computed:    true,
						},
						"endpoint": schema.StringAttribute{
							Description: "S3 endpoint the bucket lives on",
							Computed:    true,
						},
						"region": schema.StringAttribute{
							Description: "Region of the bucket",
							Computed:    true,
						},
						"bucket_name": schema.StringAttribute{
							Description: "Name of the bucket at the endpoint",
							Computed:    true,
						},
						"access_key_id": schema.StringAttribute{
							Description: "Access key ID used to reach the bucket",
							Computed:    true,
							Sensitive:   true,
						},
					},
				},
			},
		},
	}
}

func (d *S3BucketsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *S3BucketsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	buckets, err := d.client.ListS3Buckets()
	if err != nil {
		resp.Diagnostics.AddError("Failed to read S3 buckets", err.Error())
		return
	}

	data := S3BucketsDataSourceModel{
		Buckets: newS3BucketModels(buckets),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func newS3BucketModels(buckets []berth.BucketResponse) []S3BucketDataSourceModel {
	models := make([]S3BucketDataSourceModel, 0, len(buckets))
	for _, bucket := range buckets {
		models = append(models, S3BucketDataSourceModel{
			ID:          types.StringValue(strconv.FormatUint(uint64(bucket.Id), 10)),
			Label:       types.StringValue(bucket.Label),
			Endpoint:    types.StringValue(bucket.Endpoint),
			Region:      types.StringValue(bucket.Region),
			BucketName:  types.StringValue(bucket.BucketName),
			AccessKeyID: types.StringValue(bucket.AccessKeyId),
		})
	}
	return models
}
