package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	berth "github.com/tech-arch1tect/berth-go-api-client"
	"github.com/tech-arch1tect/terraform-provider-berth/internal/client"
)

var _ resource.Resource = &RegistryCredentialResource{}

func NewRegistryCredentialResource() resource.Resource {
	return &RegistryCredentialResource{}
}

type RegistryCredentialResource struct {
	client *client.Client
}

type RegistryCredentialResourceModel struct {
	ID           types.String `tfsdk:"id"`
	ServerID     types.Int64  `tfsdk:"server_id"`
	RegistryURL  types.String `tfsdk:"registry_url"`
	Username     types.String `tfsdk:"username"`
	Password     types.String `tfsdk:"password"`
	StackPattern types.String `tfsdk:"stack_pattern"`
	ImagePattern types.String `tfsdk:"image_pattern"`
}

func (r *RegistryCredentialResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_registry_credential"
}

func (r *RegistryCredentialResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a docker registry credential on a Berth server. Requires an API key carrying the registries.manage scope for the target server plus an owner user holding the registries.manage permission, an admin owner alone is not enough. Import is unsupported because the password cannot be read back from the server",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Registry credential ID",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"server_id": schema.Int64Attribute{
				Description: "Berth server the credential belongs to. Changing it replaces the credential so the stored ID is never updated against a different server",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"registry_url": schema.StringAttribute{
				Description: "Registry URL, for example host:5000. The server performs no connectivity check, so a wrong URL only surfaces when an image pull fails at deploy time",
				Required:    true,
			},
			"username": schema.StringAttribute{
				Description: "Username used to authenticate against the registry",
				Required:    true,
			},
			"password": schema.StringAttribute{
				Description: "Registry password. Write-only: the server never returns it, so it lives in Terraform state and is re-sent on every update",
				Required:    true,
				Sensitive:   true,
			},
			"stack_pattern": schema.StringAttribute{
				Description: "Stack name pattern the credential applies to. Omitting it stores the literal *, so it reads back as * and the adopted value is written back to state",
				Optional:    true,
				Computed:    true,
			},
			"image_pattern": schema.StringAttribute{
				Description: "Image name pattern the credential applies to. The server cannot distinguish an unset pattern from an empty one, so leaving it out reads back as an empty string and a pattern cannot be cleared once it is set",
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func (r *RegistryCredentialResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RegistryCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RegistryCredentialResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.createRegistryCredential(&data); err != nil {
		resp.Diagnostics.AddError("Failed to create registry credential", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RegistryCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RegistryCredentialResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	gone, err := r.refreshRegistryCredential(&data)
	switch {
	case gone:
		resp.State.RemoveResource(ctx)
		return
	case err != nil:
		resp.Diagnostics.AddError("Failed to read registry credential", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RegistryCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RegistryCredentialResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.updateRegistryCredential(&data); err != nil {
		resp.Diagnostics.AddError("Failed to update registry credential", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RegistryCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RegistryCredentialResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.deleteRegistryCredential(&data); err != nil {
		resp.Diagnostics.AddError("Failed to delete registry credential", err.Error())
		return
	}
}

func (r *RegistryCredentialResource) createRegistryCredential(data *RegistryCredentialResourceModel) error {
	if registryCredentialPassword(data) == "" {
		return errors.New("password is required to create the registry credential")
	}

	serverID, err := registryCredentialServerID(data)
	if err != nil {
		return err
	}

	credential, err := r.client.CreateRegistryCredential(
		serverID,
		registryCredentialStackPattern(data),
		data.RegistryURL.ValueString(),
		registryCredentialImagePattern(data),
		data.Username.ValueString(),
		registryCredentialPassword(data),
	)
	if err != nil {
		return err
	}
	if credential.Id == 0 {
		return errors.New("the server returned no credential ID")
	}

	data.ID = types.StringValue(strconv.FormatUint(uint64(credential.Id), 10))
	applyRegistryCredentialResponse(data, *credential)
	return nil
}

func (r *RegistryCredentialResource) refreshRegistryCredential(data *RegistryCredentialResourceModel) (bool, error) {
	serverID, err := registryCredentialServerID(data)
	if err != nil {
		return false, err
	}
	id, err := registryCredentialID(data)
	if err != nil {
		return false, err
	}

	credential, err := r.client.GetRegistryCredential(serverID, id)
	switch classifyRead(err) {
	case readGone:
		return true, nil
	case readFailed:
		return false, err
	}

	applyRegistryCredentialResponse(data, *credential)
	return false, nil
}

func (r *RegistryCredentialResource) updateRegistryCredential(data *RegistryCredentialResourceModel) error {
	if registryCredentialPassword(data) == "" {
		return errors.New("password must stay set so the stored credential keeps working")
	}

	serverID, err := registryCredentialServerID(data)
	if err != nil {
		return err
	}
	id, err := registryCredentialID(data)
	if err != nil {
		return err
	}

	credential, err := r.client.UpdateRegistryCredential(
		serverID,
		id,
		registryCredentialStackPattern(data),
		data.RegistryURL.ValueString(),
		registryCredentialImagePattern(data),
		data.Username.ValueString(),
		registryCredentialPassword(data),
	)
	if err != nil {
		return err
	}

	applyRegistryCredentialResponse(data, *credential)
	return nil
}

func (r *RegistryCredentialResource) deleteRegistryCredential(data *RegistryCredentialResourceModel) error {
	serverID, err := registryCredentialServerID(data)
	if err != nil {
		return err
	}
	id, err := registryCredentialID(data)
	if err != nil {
		return err
	}

	err = r.client.DeleteRegistryCredential(serverID, id)
	if errors.Is(err, client.ErrNotFound) {
		return nil
	}
	return err
}

func registryCredentialServerID(data *RegistryCredentialResourceModel) (uint, error) {
	value := data.ServerID.ValueInt64()
	if value <= 0 {
		return 0, errors.New("server_id must be a positive server ID")
	}
	return uint(value), nil
}

func registryCredentialID(data *RegistryCredentialResourceModel) (uint, error) {
	id, err := strconv.ParseUint(data.ID.ValueString(), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid credential ID: %w", err)
	}
	return uint(id), nil
}

func registryCredentialStackPattern(data *RegistryCredentialResourceModel) string {
	if data.StackPattern.IsNull() || data.StackPattern.IsUnknown() {
		return ""
	}
	return data.StackPattern.ValueString()
}

func registryCredentialImagePattern(data *RegistryCredentialResourceModel) string {
	if data.ImagePattern.IsNull() || data.ImagePattern.IsUnknown() {
		return ""
	}
	return data.ImagePattern.ValueString()
}

func registryCredentialPassword(data *RegistryCredentialResourceModel) string {
	if data.Password.IsNull() || data.Password.IsUnknown() {
		return ""
	}
	return data.Password.ValueString()
}

func applyRegistryCredentialResponse(data *RegistryCredentialResourceModel, credential berth.RegistryCredentialInfo) {
	data.ServerID = types.Int64Value(int64(credential.ServerId))
	data.RegistryURL = types.StringValue(credential.RegistryUrl)
	data.Username = types.StringValue(credential.Username)
	data.StackPattern = types.StringValue(credential.StackPattern)
	data.ImagePattern = types.StringValue("")
	if credential.ImagePattern != nil {
		data.ImagePattern = types.StringValue(*credential.ImagePattern)
	}
}
