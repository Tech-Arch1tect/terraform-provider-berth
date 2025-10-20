package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tech-arch1tect/terraform-provider-berth/internal/client"
)

var _ provider.Provider = &BerthProvider{}

type BerthProvider struct {
	version string
}

type BerthProviderModel struct {
	URL                types.String `tfsdk:"url"`
	APIKey             types.String `tfsdk:"api_key"`
	InsecureSkipVerify types.Bool   `tfsdk:"insecure_skip_verify"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &BerthProvider{
			version: version,
		}
	}
}

func (p *BerthProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "berth"
	resp.Version = p.version
}

func (p *BerthProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for managing Berth roles and permissions",
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				Description: "Berth server URL (e.g., https://berth.example.com)",
				Required:    true,
			},
			"api_key": schema.StringAttribute{
				Description: "Berth API key (must have admin privileges)",
				Required:    true,
				Sensitive:   true,
			},
			"insecure_skip_verify": schema.BoolAttribute{
				Description: "Skip TLS certificate verification",
				Optional:    true,
			},
		},
	}
}

func (p *BerthProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config BerthProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	insecureSkipVerify := false
	if !config.InsecureSkipVerify.IsNull() {
		insecureSkipVerify = config.InsecureSkipVerify.ValueBool()
	}

	client := client.NewClient(
		config.URL.ValueString(),
		config.APIKey.ValueString(),
		insecureSkipVerify,
	)

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *BerthProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewRoleResource,
		NewRolePermissionResource,
	}
}

func (p *BerthProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}
