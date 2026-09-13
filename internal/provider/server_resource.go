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

var _ resource.Resource = &ServerResource{}

func NewServerResource() resource.Resource {
	return &ServerResource{}
}

type ServerResource struct {
	client *client.Client
}

type ServerResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Host                types.String `tfsdk:"host"`
	Port                types.Int64  `tfsdk:"port"`
	Description         types.String `tfsdk:"description"`
	AccessToken         types.String `tfsdk:"access_token"`
	BackupPassword      types.String `tfsdk:"backup_password"`
	SkipSSLVerification types.Bool   `tfsdk:"skip_ssl_verification"`
	IsActive            types.Bool   `tfsdk:"is_active"`
	BackupsEnabled      types.Bool   `tfsdk:"backups_enabled"`
	S3BucketID          types.Int64  `tfsdk:"s3_bucket_id"`
}

func (r *ServerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server"
}

func (r *ServerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Berth server registration through the admin servers endpoint. Requires an admin token with the admin.servers.write scope. Import is unsupported because the agent access token cannot be read back from the server",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Server ID",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Display name of the server",
				Required:    true,
			},
			"host": schema.StringAttribute{
				Description: "Host the Berth agent runs on. The server performs no format or reachability check, so a wrong host only surfaces when the server next tries to reach the agent",
				Required:    true,
			},
			"port": schema.Int64Attribute{
				Description: "Port the Berth agent listens on, greater than zero",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Free-text description of the server",
				Optional:    true,
				Computed:    true,
			},
			"access_token": schema.StringAttribute{
				Description: "Agent access token. Write-only: the server never returns it, so it lives in Terraform state and is re-sent on every update; a blank value keeps the stored token. A wrong token only surfaces when the server next connects to the agent",
				Required:    true,
				Sensitive:   true,
			},
			"backup_password": schema.StringAttribute{
				Description: "Password protecting the server's stack backups. Write-only: the server never returns it, so it lives in Terraform state and is re-sent on every update; a blank value keeps the stored one. The server requires it whenever backups are enabled, so enabling backups without a stored or new password fails before any request is sent, and once set it cannot be cleared, only replaced",
				Optional:    true,
				Sensitive:   true,
			},
			"skip_ssl_verification": schema.BoolAttribute{
				Description: "Whether TLS certificate verification is skipped for the agent connection. Omitting it on create stores no value, which the server reads back as true",
				Optional:    true,
				Computed:    true,
			},
			"is_active": schema.BoolAttribute{
				Description: "Whether the server is active for users. Setting it to false deactivates the server. The value is sent explicitly on every update so an omitted one never silently deactivates it. The server ignores a false value on create and reads a new server back as active, so deactivate it through an update",
				Optional:    true,
				Computed:    true,
			},
			"backups_enabled": schema.BoolAttribute{
				Description: "Whether stack backups are enabled for the server",
				Optional:    true,
				Computed:    true,
			},
			"s3_bucket_id": schema.Int64Attribute{
				Description: "ID of the S3 bucket assigned for the server's stack backups, null when none is assigned. Removing the attribute clears the assignment, which the server rejects while that bucket still holds backups. The server accepts no bucket on create, so assigning one at create time issues an update straight after; if that update fails, the server registration exists but Terraform holds no state, and retrying the apply registers a second server",
				Optional:    true,
			},
		},
	}
}

func (r *ServerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServerResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.createServer(&data); err != nil {
		resp.Diagnostics.AddError("Failed to create server", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServerResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	gone, err := r.refreshServer(&data)
	switch {
	case gone:
		resp.State.RemoveResource(ctx)
		return
	case err != nil:
		resp.Diagnostics.AddError("Failed to read server", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ServerResourceModel
	var state ServerResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.updateServer(&plan, &state); err != nil {
		resp.Diagnostics.AddError("Failed to update server", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServerResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.deleteServer(&data); err != nil {
		resp.Diagnostics.AddError("Failed to delete server", err.Error())
		return
	}
}

func (r *ServerResource) createServer(data *ServerResourceModel) error {
	port, backupsEnabled, isActive, skipSSLVerification, err := serverWriteInput(data)
	if err != nil {
		return err
	}
	if backupsEnabled != nil && *backupsEnabled && serverBackupPassword(data) == "" {
		return errors.New("backup_password is required when backups_enabled is true")
	}

	server, err := r.client.CreateServer(
		data.Name.ValueString(),
		data.Host.ValueString(),
		port,
		data.AccessToken.ValueString(),
		serverBackupPassword(data),
		serverDescription(data),
		backupsEnabled,
		isActive,
		skipSSLVerification,
	)
	if err != nil {
		return err
	}
	if server.Id == 0 {
		return errors.New("the server returned no server ID")
	}

	data.ID = types.StringValue(strconv.FormatUint(uint64(server.Id), 10))
	plannedBucketID := data.S3BucketID
	applyServerResponse(data, *server)

	if plannedBucketID.IsNull() {
		return nil
	}

	state := *data
	state.S3BucketID = types.Int64Null()
	data.S3BucketID = plannedBucketID
	return r.updateServer(data, &state)
}

func (r *ServerResource) refreshServer(data *ServerResourceModel) (bool, error) {
	id, err := serverResourceID(data)
	if err != nil {
		return false, err
	}

	server, err := r.client.GetServer(id)
	switch classifyRead(err) {
	case readGone:
		return true, nil
	case readFailed:
		return false, err
	}

	applyServerResponse(data, *server)
	return false, nil
}

func (r *ServerResource) updateServer(data, state *ServerResourceModel) error {
	port, backupsEnabled, isActive, skipSSLVerification, err := serverWriteInput(data)
	if err != nil {
		return err
	}
	if backupsEnabled != nil && *backupsEnabled && serverBackupPassword(data) == "" && serverBackupPassword(state) == "" {
		return errors.New("backup_password is required when backups_enabled is true and no password is stored on the server yet")
	}

	id, err := serverResourceID(data)
	if err != nil {
		return err
	}

	server, err := r.client.UpdateServer(
		id,
		data.Name.ValueString(),
		data.Host.ValueString(),
		port,
		data.AccessToken.ValueString(),
		serverBackupPassword(data),
		serverDescription(data),
		backupsEnabled,
		isActive,
		skipSSLVerification,
		serverOptionalInt64(data.S3BucketID),
	)
	if err != nil {
		return err
	}

	applyServerResponse(data, *server)
	return nil
}

func (r *ServerResource) deleteServer(data *ServerResourceModel) error {
	id, err := serverResourceID(data)
	if err != nil {
		return err
	}

	err = r.client.DeleteServer(id)
	if errors.Is(err, client.ErrNotFound) {
		return nil
	}
	return err
}

func serverWriteInput(data *ServerResourceModel) (int64, *bool, *bool, *bool, error) {
	port := data.Port.ValueInt64()
	if port <= 0 {
		return 0, nil, nil, nil, errors.New("port must be greater than zero")
	}
	return port, serverOptionalBool(data.BackupsEnabled), serverOptionalBool(data.IsActive), serverOptionalBool(data.SkipSSLVerification), nil
}

func serverResourceID(data *ServerResourceModel) (uint, error) {
	id, err := strconv.ParseUint(data.ID.ValueString(), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid server ID: %w", err)
	}
	return uint(id), nil
}

func serverDescription(data *ServerResourceModel) string {
	if data.Description.IsNull() || data.Description.IsUnknown() {
		return ""
	}
	return data.Description.ValueString()
}

func serverBackupPassword(data *ServerResourceModel) string {
	if data.BackupPassword.IsNull() || data.BackupPassword.IsUnknown() {
		return ""
	}
	return data.BackupPassword.ValueString()
}

func serverOptionalBool(value types.Bool) *bool {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	boolValue := value.ValueBool()
	return &boolValue
}

func serverOptionalInt64(value types.Int64) *int64 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	intValue := value.ValueInt64()
	return &intValue
}

func applyServerResponse(data *ServerResourceModel, server berth.ServerInfo) {
	data.Name = types.StringValue(server.Name)
	data.Host = types.StringValue(server.Host)
	data.Port = types.Int64Value(int64(server.Port))
	data.Description = types.StringValue(server.Description)
	data.SkipSSLVerification = types.BoolValue(server.SkipSslVerification)
	data.IsActive = types.BoolValue(server.IsActive)
	data.BackupsEnabled = types.BoolValue(server.BackupsEnabled)
	data.S3BucketID = nullableInt64(server.S3BucketId.Get())
}
