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

var _ datasource.DataSource = &ServersDataSource{}

func NewServersDataSource() datasource.DataSource {
	return &ServersDataSource{}
}

type ServersDataSource struct {
	client *client.Client
}

type ServersDataSourceModel struct {
	Servers []ServerDataSourceModel `tfsdk:"servers"`
}

type ServerDataSourceModel struct {
	ID                            types.String `tfsdk:"id"`
	Name                          types.String `tfsdk:"name"`
	Description                   types.String `tfsdk:"description"`
	Host                          types.String `tfsdk:"host"`
	Port                          types.Int64  `tfsdk:"port"`
	SkipSSLVerification           types.Bool   `tfsdk:"skip_ssl_verification"`
	IsActive                      types.Bool   `tfsdk:"is_active"`
	BackupsEnabled                types.Bool   `tfsdk:"backups_enabled"`
	S3BucketID                    types.Int64  `tfsdk:"s3_bucket_id"`
	AgentCertFingerprint          types.String `tfsdk:"agent_cert_fingerprint"`
	AgentCertIssuedAt             types.String `tfsdk:"agent_cert_issued_at"`
	AgentCertExpiresAt            types.String `tfsdk:"agent_cert_expires_at"`
	AgentCertAuthorityFingerprint types.String `tfsdk:"agent_cert_authority_fingerprint"`
}

func (d *ServersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_servers"
}

func (d *ServersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Every Berth server registered with the admin servers endpoint. Agent credentials and backup passwords are never part of the response. Requires an admin token with the admin.servers.read scope",
		Attributes: map[string]schema.Attribute{
			"servers": schema.ListNestedAttribute{
				Description: "All servers, ordered by ID as the server returns them",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "Server ID",
							Computed:    true,
						},
						"name": schema.StringAttribute{
							Description: "Server name",
							Computed:    true,
						},
						"description": schema.StringAttribute{
							Description: "Server description",
							Computed:    true,
						},
						"host": schema.StringAttribute{
							Description: "Host the Berth agent runs on",
							Computed:    true,
						},
						"port": schema.Int64Attribute{
							Description: "Port the Berth agent listens on",
							Computed:    true,
						},
						"skip_ssl_verification": schema.BoolAttribute{
							Description: "Whether TLS certificate verification is skipped for the agent connection",
							Computed:    true,
						},
						"is_active": schema.BoolAttribute{
							Description: "Whether the server is active",
							Computed:    true,
						},
						"backups_enabled": schema.BoolAttribute{
							Description: "Whether stack backups are enabled for the server",
							Computed:    true,
						},
						"s3_bucket_id": schema.Int64Attribute{
							Description: "ID of the S3 bucket assigned for backups, null when none is assigned",
							Computed:    true,
						},
						"agent_cert_fingerprint": schema.StringAttribute{
							Description: "Fingerprint of the agent certificate, null when no certificate has been issued",
							Computed:    true,
						},
						"agent_cert_issued_at": schema.StringAttribute{
							Description: "Issue timestamp of the agent certificate, null when no certificate has been issued",
							Computed:    true,
						},
						"agent_cert_expires_at": schema.StringAttribute{
							Description: "Expiry timestamp of the agent certificate, null when no certificate has been issued",
							Computed:    true,
						},
						"agent_cert_authority_fingerprint": schema.StringAttribute{
							Description: "Fingerprint of the agent certificate authority, null when no certificate has been issued",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *ServersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ServersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	servers, err := d.client.ListServers()
	if err != nil {
		resp.Diagnostics.AddError("Failed to read servers", err.Error())
		return
	}

	data := ServersDataSourceModel{
		Servers: newServerModels(servers),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func newServerModels(servers []berth.ServerInfo) []ServerDataSourceModel {
	models := make([]ServerDataSourceModel, 0, len(servers))
	for _, server := range servers {
		models = append(models, ServerDataSourceModel{
			ID:                            types.StringValue(strconv.FormatUint(uint64(server.Id), 10)),
			Name:                          types.StringValue(server.Name),
			Description:                   types.StringValue(server.Description),
			Host:                          types.StringValue(server.Host),
			Port:                          types.Int64Value(int64(server.Port)),
			SkipSSLVerification:           types.BoolValue(server.SkipSslVerification),
			IsActive:                      types.BoolValue(server.IsActive),
			BackupsEnabled:                types.BoolValue(server.BackupsEnabled),
			S3BucketID:                    nullableInt64(server.S3BucketId.Get()),
			AgentCertFingerprint:          nullableString(server.AgentCertFingerprint),
			AgentCertIssuedAt:             nullableString(server.AgentCertIssuedAt),
			AgentCertExpiresAt:            nullableString(server.AgentCertExpiresAt),
			AgentCertAuthorityFingerprint: nullableString(server.AgentCertAuthorityFingerprint),
		})
	}
	return models
}

func nullableInt64(value *int32) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*value))
}

func nullableString(value *string) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(*value)
}
