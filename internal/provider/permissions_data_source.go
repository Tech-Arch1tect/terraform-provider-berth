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

var _ datasource.DataSource = &PermissionsDataSource{}

func NewPermissionsDataSource() datasource.DataSource {
	return &PermissionsDataSource{}
}

type PermissionsDataSource struct {
	client *client.Client
}

type PermissionsDataSourceModel struct {
	Permissions []PermissionDataSourceModel `tfsdk:"permissions"`
}

type PermissionDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Resource     types.String `tfsdk:"resource"`
	Action       types.String `tfsdk:"action"`
	Description  types.String `tfsdk:"description"`
	IsAPIKeyOnly types.Bool   `tfsdk:"is_api_key_only"`
}

func (d *PermissionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_permissions"
}

func (d *PermissionsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Every permission registered on the Berth server. Requires an admin token with the admin.permissions.read scope",
		Attributes: map[string]schema.Attribute{
			"permissions": schema.ListNestedAttribute{
				Description: "All permissions, ordered by ID as the server returns them",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "Permission ID",
							Computed:    true,
						},
						"name": schema.StringAttribute{
							Description: "Permission name (e.g., 'stacks.read')",
							Computed:    true,
						},
						"resource": schema.StringAttribute{
							Description: "Resource the permission guards (e.g., 'stacks')",
							Computed:    true,
						},
						"action": schema.StringAttribute{
							Description: "Action the permission allows (e.g., 'read', 'manage')",
							Computed:    true,
						},
						"description": schema.StringAttribute{
							Description: "Human readable description of the permission",
							Computed:    true,
						},
						"is_api_key_only": schema.BoolAttribute{
							Description: "Whether the permission is grantable only through API keys",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *PermissionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PermissionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	permissions, err := d.client.ListPermissions()
	if err != nil {
		resp.Diagnostics.AddError("Failed to read permissions", err.Error())
		return
	}

	data := PermissionsDataSourceModel{
		Permissions: newPermissionModels(permissions),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func newPermissionModels(permissions []berth.PermissionInfo) []PermissionDataSourceModel {
	models := make([]PermissionDataSourceModel, 0, len(permissions))
	for _, permission := range permissions {
		models = append(models, PermissionDataSourceModel{
			ID:           types.StringValue(strconv.FormatUint(uint64(permission.Id), 10)),
			Name:         types.StringValue(permission.Name),
			Resource:     types.StringValue(permission.Resource),
			Action:       types.StringValue(permission.Action),
			Description:  types.StringValue(permission.Description),
			IsAPIKeyOnly: types.BoolValue(permission.IsApiKeyOnly),
		})
	}
	return models
}
