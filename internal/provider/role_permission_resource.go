package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tech-arch1tect/terraform-provider-berth/internal/client"
)

var _ resource.Resource = &RolePermissionResource{}
var _ resource.ResourceWithImportState = &RolePermissionResource{}

func NewRolePermissionResource() resource.Resource {
	return &RolePermissionResource{}
}

type RolePermissionResource struct {
	client *client.Client
}

type RolePermissionResourceModel struct {
	ID             types.String `tfsdk:"id"`
	RoleID         types.Int64  `tfsdk:"role_id"`
	ServerID       types.Int64  `tfsdk:"server_id"`
	PermissionName types.String `tfsdk:"permission_name"`
	StackPattern   types.String `tfsdk:"stack_pattern"`
}

func (r *RolePermissionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role_permission"
}

func (r *RolePermissionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Berth role permission for server/stack access",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Permission ID",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"role_id": schema.Int64Attribute{
				Description: "Role ID",
				Required:    true,
			},
			"server_id": schema.Int64Attribute{
				Description: "Server ID",
				Required:    true,
			},
			"permission_name": schema.StringAttribute{
				Description: "Permission name (e.g., 'stacks.read', 'stacks.manage', 'stacks.create', 'files.read', 'files.write', 'logs.read')",
				Required:    true,
			},
			"stack_pattern": schema.StringAttribute{
				Description: "Stack name pattern (supports wildcards, e.g., '*', 'prod-*')",
				Optional:    true,
			},
		},
	}
}

func (r *RolePermissionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RolePermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RolePermissionResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	permission, err := r.client.GetPermissionByName(data.PermissionName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to find permission", err.Error())
		return
	}

	stackPattern := "*"
	if !data.StackPattern.IsNull() {
		stackPattern = data.StackPattern.ValueString()
	}

	perm, err := r.client.CreateRolePermission(
		uint(data.RoleID.ValueInt64()),
		uint(data.ServerID.ValueInt64()),
		permission.ID,
		stackPattern,
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create role permission", err.Error())
		return
	}

	perms, _, err := r.client.ListRolePermissions(uint(data.RoleID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to read created permission", err.Error())
		return
	}

	var foundID uint
	for _, p := range perms {
		if p.ServerID == perm.ServerID && p.PermissionID == permission.ID && p.StackPattern == stackPattern {
			foundID = p.ID
			break
		}
	}

	if foundID == 0 {
		resp.Diagnostics.AddError("Failed to find created permission", "Permission was created but could not be found")
		return
	}

	data.ID = types.StringValue(strconv.FormatUint(uint64(foundID), 10))
	data.StackPattern = types.StringValue(stackPattern)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RolePermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RolePermissionResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseUint(data.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid permission ID", err.Error())
		return
	}

	perm, err := r.client.GetRolePermission(uint(data.RoleID.ValueInt64()), uint(id))
	if err != nil {
		resp.Diagnostics.AddError("Failed to read role permission", err.Error())
		return
	}

	data.ServerID = types.Int64Value(int64(perm.ServerID))
	data.StackPattern = types.StringValue(perm.StackPattern)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RolePermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Role permissions cannot be updated. Please delete and recreate the resource.",
	)
}

func (r *RolePermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RolePermissionResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseUint(data.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid permission ID", err.Error())
		return
	}

	if err := r.client.DeleteRolePermission(uint(data.RoleID.ValueInt64()), uint(id)); err != nil {
		resp.Diagnostics.AddError("Failed to delete role permission", err.Error())
		return
	}
}

func (r *RolePermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"Import ID must be in format 'role_id:permission_id'",
		)
		return
	}

	roleID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid role ID", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role_id"), roleID)...)
}
