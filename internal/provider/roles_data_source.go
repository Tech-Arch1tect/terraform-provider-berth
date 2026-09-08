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

var _ datasource.DataSource = &RolesDataSource{}

func NewRolesDataSource() datasource.DataSource {
	return &RolesDataSource{}
}

type RolesDataSource struct {
	client *client.Client
}

type RolesDataSourceModel struct {
	Roles []RoleDataSourceModel `tfsdk:"roles"`
}

type RoleDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	IsAdmin     types.Bool   `tfsdk:"is_admin"`
}

func (d *RolesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_roles"
}

func (d *RolesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Every Berth role. The roles endpoint always returns an empty permissions array for each role, so role permissions are deliberately not part of this data source; inspect them through the berth_role resource or the role rule endpoints. Requires an admin token with the admin.roles.read scope",
		Attributes: map[string]schema.Attribute{
			"roles": schema.ListNestedAttribute{
				Description: "All roles, ordered by ID as the server returns them",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "Role ID",
							Computed:    true,
						},
						"name": schema.StringAttribute{
							Description: "Role name",
							Computed:    true,
						},
						"description": schema.StringAttribute{
							Description: "Role description",
							Computed:    true,
						},
						"is_admin": schema.BoolAttribute{
							Description: "Whether the role carries unrestricted admin access",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *RolesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RolesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	roles, err := d.client.ListRoles()
	if err != nil {
		resp.Diagnostics.AddError("Failed to read roles", err.Error())
		return
	}

	data := RolesDataSourceModel{
		Roles: newRoleModels(roles),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func newRoleModels(roles []berth.RoleWithPermissions) []RoleDataSourceModel {
	models := make([]RoleDataSourceModel, 0, len(roles))
	for _, role := range roles {
		models = append(models, RoleDataSourceModel{
			ID:          types.StringValue(strconv.FormatUint(uint64(role.Id), 10)),
			Name:        types.StringValue(role.Name),
			Description: types.StringValue(role.Description),
			IsAdmin:     types.BoolValue(role.IsAdmin),
		})
	}
	return models
}
