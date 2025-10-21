package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tech-arch1tect/terraform-provider-berth/internal/client"
)

var _ resource.Resource = &RoleResource{}
var _ resource.ResourceWithImportState = &RoleResource{}

func NewRoleResource() resource.Resource {
	return &RoleResource{}
}

type RoleResource struct {
	client *client.Client
}

type RoleResourceModel struct {
	ID             types.String           `tfsdk:"id"`
	Name           types.String           `tfsdk:"name"`
	Description    types.String           `tfsdk:"description"`
	Permissions    []RolePermissionInline `tfsdk:"permissions"`
	PermissionSets []PermissionSet        `tfsdk:"permission_set"`
}

type RolePermissionInline struct {
	ID             types.String `tfsdk:"id"`
	ServerID       types.Int64  `tfsdk:"server_id"`
	PermissionName types.String `tfsdk:"permission_name"`
	StackPattern   types.String `tfsdk:"stack_pattern"`
}

type PermissionSet struct {
	ServerIDs   []types.Int64          `tfsdk:"server_ids"`
	Permissions []PermissionDefinition `tfsdk:"permissions"`
}

type PermissionDefinition struct {
	Name    types.String `tfsdk:"name"`
	Pattern types.String `tfsdk:"pattern"`
}

func (r *RoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Berth role with optional inline permissions",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Role ID",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Role name (must be unique)",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Role description",
				Optional:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"permissions": schema.ListNestedBlock{
				Description: "Inline permissions for this role (use permission_set for bulk assignment to multiple servers)",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "Permission ID",
							Computed:    true,
						},
						"server_id": schema.Int64Attribute{
							Description: "Server ID",
							Required:    true,
						},
						"permission_name": schema.StringAttribute{
							Description: "Permission name (e.g., 'stacks.read', 'stacks.manage', 'files.read', 'files.write', 'logs.read')",
							Required:    true,
						},
						"stack_pattern": schema.StringAttribute{
							Description: "Stack name pattern (supports wildcards, e.g., '*', 'prod-*'). Defaults to '*'",
							Optional:    true,
							Computed:    true,
						},
					},
				},
			},
			"permission_set": schema.ListNestedBlock{
				Description: "Permission sets - apply multiple permissions to multiple servers at once",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"server_ids": schema.ListAttribute{
							Description: "List of server IDs to apply these permissions to",
							Required:    true,
							ElementType: types.Int64Type,
						},
					},
					Blocks: map[string]schema.Block{
						"permissions": schema.ListNestedBlock{
							Description: "List of permissions to apply",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Description: "Permission name (e.g., 'stacks.read', 'stacks.manage')",
										Required:    true,
									},
									"pattern": schema.StringAttribute{
										Description: "Stack pattern. Defaults to '*'",
										Optional:    true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *RoleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RoleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := r.client.CreateRole(data.Name.ValueString(), data.Description.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to create role", err.Error())
		return
	}

	data.ID = types.StringValue(strconv.FormatUint(uint64(role.ID), 10))

	for _, permSet := range data.PermissionSets {
		for _, serverID := range permSet.ServerIDs {
			for _, perm := range permSet.Permissions {
				stackPattern := "*"
				if !perm.Pattern.IsNull() && !perm.Pattern.IsUnknown() {
					stackPattern = perm.Pattern.ValueString()
				}

				permission, err := r.client.GetPermissionByName(perm.Name.ValueString())
				if err != nil {
					resp.Diagnostics.AddError("Failed to find permission", err.Error())
					return
				}

				_, err = r.client.CreateRolePermission(
					role.ID,
					uint(serverID.ValueInt64()),
					permission.ID,
					stackPattern,
				)
				if err != nil {
					resp.Diagnostics.AddError("Failed to create role permission from permission set", err.Error())
					return
				}
			}
		}
	}

	for i, perm := range data.Permissions {
		stackPattern := "*"
		if !perm.StackPattern.IsNull() && !perm.StackPattern.IsUnknown() {
			stackPattern = perm.StackPattern.ValueString()
		}

		permission, err := r.client.GetPermissionByName(perm.PermissionName.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to find permission", err.Error())
			return
		}

		createdPerm, err := r.client.CreateRolePermission(
			role.ID,
			uint(perm.ServerID.ValueInt64()),
			permission.ID,
			stackPattern,
		)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create role permission", err.Error())
			return
		}

		perms, _, err := r.client.ListRolePermissions(role.ID)
		if err != nil {
			resp.Diagnostics.AddError("Failed to read created permission", err.Error())
			return
		}

		for _, p := range perms {
			if p.ServerID == createdPerm.ServerID && p.PermissionID == permission.ID && p.StackPattern == stackPattern {
				data.Permissions[i].ID = types.StringValue(strconv.FormatUint(uint64(p.ID), 10))
				data.Permissions[i].StackPattern = types.StringValue(stackPattern)
				break
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseUint(data.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid role ID", err.Error())
		return
	}

	role, err := r.client.GetRole(uint(id))
	if err != nil {
		resp.Diagnostics.AddError("Failed to read role", err.Error())
		return
	}

	data.Name = types.StringValue(role.Name)
	data.Description = types.StringValue(role.Description)

	if len(data.Permissions) > 0 {
		perms, allPermissions, err := r.client.ListRolePermissions(uint(id))
		if err != nil {
			resp.Diagnostics.AddError("Failed to read role permissions", err.Error())
			return
		}

		permMap := make(map[uint]string)
		for _, p := range allPermissions {
			permMap[p.ID] = p.Name
		}

		updatedPerms := make([]RolePermissionInline, 0, len(perms))
		for _, perm := range perms {
			updatedPerms = append(updatedPerms, RolePermissionInline{
				ID:             types.StringValue(strconv.FormatUint(uint64(perm.ID), 10)),
				ServerID:       types.Int64Value(int64(perm.ServerID)),
				PermissionName: types.StringValue(permMap[perm.PermissionID]),
				StackPattern:   types.StringValue(perm.StackPattern),
			})
		}
		data.Permissions = updatedPerms
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, state RoleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseUint(data.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid role ID", err.Error())
		return
	}

	roleID := uint(id)

	_, err = r.client.UpdateRole(roleID, data.Name.ValueString(), data.Description.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to update role", err.Error())
		return
	}

	if len(data.Permissions) > 0 || len(state.Permissions) > 0 || len(data.PermissionSets) > 0 || len(state.PermissionSets) > 0 {

		existingPerms, _, err := r.client.ListRolePermissions(roleID)
		if err != nil {
			resp.Diagnostics.AddError("Failed to read existing permissions", err.Error())
			return
		}

		for _, perm := range existingPerms {
			if err := r.client.DeleteRolePermission(roleID, perm.ID); err != nil {
				resp.Diagnostics.AddError("Failed to delete permission", err.Error())
				return
			}
		}

		for _, permSet := range data.PermissionSets {
			for _, serverID := range permSet.ServerIDs {
				for _, perm := range permSet.Permissions {
					stackPattern := "*"
					if !perm.Pattern.IsNull() && !perm.Pattern.IsUnknown() {
						stackPattern = perm.Pattern.ValueString()
					}

					permission, err := r.client.GetPermissionByName(perm.Name.ValueString())
					if err != nil {
						resp.Diagnostics.AddError("Failed to find permission", err.Error())
						return
					}

					_, err = r.client.CreateRolePermission(
						roleID,
						uint(serverID.ValueInt64()),
						permission.ID,
						stackPattern,
					)
					if err != nil {
						resp.Diagnostics.AddError("Failed to create role permission from permission set", err.Error())
						return
					}
				}
			}
		}

		for i, perm := range data.Permissions {
			stackPattern := "*"
			if !perm.StackPattern.IsNull() && !perm.StackPattern.IsUnknown() {
				stackPattern = perm.StackPattern.ValueString()
			}

			permission, err := r.client.GetPermissionByName(perm.PermissionName.ValueString())
			if err != nil {
				resp.Diagnostics.AddError("Failed to find permission", err.Error())
				return
			}

			createdPerm, err := r.client.CreateRolePermission(
				roleID,
				uint(perm.ServerID.ValueInt64()),
				permission.ID,
				stackPattern,
			)
			if err != nil {
				resp.Diagnostics.AddError("Failed to create role permission", err.Error())
				return
			}

			perms, _, err := r.client.ListRolePermissions(roleID)
			if err != nil {
				resp.Diagnostics.AddError("Failed to read created permission", err.Error())
				return
			}

			for _, p := range perms {
				if p.ServerID == createdPerm.ServerID && p.PermissionID == permission.ID && p.StackPattern == stackPattern {
					data.Permissions[i].ID = types.StringValue(strconv.FormatUint(uint64(p.ID), 10))
					data.Permissions[i].StackPattern = types.StringValue(stackPattern)
					break
				}
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RoleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseUint(data.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid role ID", err.Error())
		return
	}

	if err := r.client.DeleteRole(uint(id)); err != nil {
		resp.Diagnostics.AddError("Failed to delete role", err.Error())
		return
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
