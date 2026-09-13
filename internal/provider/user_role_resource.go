package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	berth "github.com/tech-arch1tect/berth-go-api-client"
	"github.com/tech-arch1tect/terraform-provider-berth/internal/client"
)

var _ resource.Resource = &UserRoleResource{}
var _ resource.ResourceWithImportState = &UserRoleResource{}

func NewUserRoleResource() resource.Resource {
	return &UserRoleResource{}
}

type UserRoleResource struct {
	client *client.Client
}

type UserRoleResourceModel struct {
	ID     types.String `tfsdk:"id"`
	UserID types.Int64  `tfsdk:"user_id"`
	RoleID types.Int64  `tfsdk:"role_id"`
}

func (r *UserRoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_role"
}

func (r *UserRoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a single Berth role assignment for a user. Requires an API key carrying both the admin.users.write scope, to assign and revoke the role, and the admin.users.read scope, which the read-back that verifies each assignment needs, so a key holding only admin.users.write is not sufficient. The server permits assigning the administrator role here, only its last-administrator rule protects it, so keep the administrator role out of Terraform. The resource is a pure association: the user and the role must already exist and neither is managed by it",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "User role assignment ID in the format 'user_id:role_id'",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_id": schema.Int64Attribute{
				Description: "Berth user ID the role is granted to",
				Required:    true,
			},
			"role_id": schema.Int64Attribute{
				Description: "Berth role ID granted to the user",
				Required:    true,
			},
		},
	}
}

func (r *UserRoleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *UserRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserRoleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.createUserRole(&data); err != nil {
		resp.Diagnostics.AddError("Failed to create user role assignment", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserRoleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	gone, err := r.refreshUserRole(&data)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read user role assignment", err.Error())
		return
	}
	if gone {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"User role assignments cannot be updated. Please delete and recreate the resource.",
	)
}

func (r *UserRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserRoleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.deleteUserRole(&data); err != nil {
		resp.Diagnostics.AddError("Failed to delete user role assignment", err.Error())
		return
	}
}

func (r *UserRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	userID, roleID, err := parseUserRoleImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), userRoleID(userID, roleID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), int64(userID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role_id"), int64(roleID))...)
}

func (r *UserRoleResource) createUserRole(data *UserRoleResourceModel) error {
	userID, err := userRoleUserID(data)
	if err != nil {
		return err
	}
	roleID, err := userRoleRoleID(data)
	if err != nil {
		return err
	}

	if err := r.client.AssignRole(userID, roleID); err != nil {
		return err
	}

	user, err := r.client.GetUserRoles(userID)
	if err != nil {
		return err
	}
	if !userRolePresent(user, roleID) {
		return fmt.Errorf("role %d was assigned but the user's roles do not list it", roleID)
	}

	data.ID = types.StringValue(userRoleID(userID, roleID))
	return nil
}

func (r *UserRoleResource) refreshUserRole(data *UserRoleResourceModel) (bool, error) {
	userID, err := userRoleUserID(data)
	if err != nil {
		return false, err
	}
	roleID, err := userRoleRoleID(data)
	if err != nil {
		return false, err
	}

	user, err := r.client.GetUserRoles(userID)
	switch classifyRead(err) {
	case readGone:
		return true, nil
	case readFailed:
		return false, err
	}

	if !userRolePresent(user, roleID) {
		return true, nil
	}

	data.ID = types.StringValue(userRoleID(userID, roleID))
	return false, nil
}

func (r *UserRoleResource) deleteUserRole(data *UserRoleResourceModel) error {
	userID, err := userRoleUserID(data)
	if err != nil {
		return err
	}
	roleID, err := userRoleRoleID(data)
	if err != nil {
		return err
	}

	err = r.client.RevokeRole(userID, roleID)
	if errors.Is(err, client.ErrNotFound) {
		return nil
	}
	return err
}

func userRoleUserID(data *UserRoleResourceModel) (uint, error) {
	value := data.UserID.ValueInt64()
	if value <= 0 {
		return 0, errors.New("user_id must be a positive user ID")
	}
	return uint(value), nil
}

func userRoleRoleID(data *UserRoleResourceModel) (uint, error) {
	value := data.RoleID.ValueInt64()
	if value <= 0 {
		return 0, errors.New("role_id must be a positive role ID")
	}
	return uint(value), nil
}

func userRolePresent(user *berth.UserInfo, roleID uint) bool {
	if user == nil {
		return false
	}
	for _, role := range user.GetRoles() {
		if uint(role.Id) == roleID {
			return true
		}
	}
	return false
}

func userRoleID(userID, roleID uint) string {
	return fmt.Sprintf("%d:%d", userID, roleID)
}

func parseUserRoleImportID(id string) (uint, uint, error) {
	parts := strings.Split(id, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("import ID must be in the format 'user_id:role_id', got %q", id)
	}

	userID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid user ID in import ID %q: %w", id, err)
	}

	roleID, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid role ID in import ID %q: %w", id, err)
	}

	return uint(userID), uint(roleID), nil
}
