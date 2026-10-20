// Package provider implements the Privatus Analytics Terraform provider.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/privatus-analytics/terraform-provider-privatus/internal/client"
)

var _ provider.Provider = &privatusProvider{}

type privatusProvider struct {
	version string
}

type providerModel struct {
	APIToken types.String `tfsdk:"api_token"`
	Endpoint types.String `tfsdk:"endpoint"`
}

// New returns a provider factory.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &privatusProvider{version: version}
	}
}

func (p *privatusProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "privatus"
	resp.Version = p.version
}

func (p *privatusProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Privatus Analytics (privacy-first, cookieless web analytics) sites, goals and traffic rules.",
		Attributes: map[string]schema.Attribute{
			"api_token": schema.StringAttribute{
				Description: "Personal API token (`pat_…`). Can also be set with the `PRIVATUS_API_TOKEN` environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"endpoint": schema.StringAttribute{
				Description: "API endpoint (base URL). Defaults to the `PRIVATUS_ENDPOINT` environment variable or `https://privatusanalytics.com`.",
				Optional:    true,
			},
		},
	}
}

func (p *privatusProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg.APIToken.IsUnknown() || cfg.Endpoint.IsUnknown() {
		// Values will be known at apply time; defer configuration.
		return
	}

	token := os.Getenv("PRIVATUS_API_TOKEN")
	if !cfg.APIToken.IsNull() && cfg.APIToken.ValueString() != "" {
		token = cfg.APIToken.ValueString()
	}
	baseURL := os.Getenv("PRIVATUS_ENDPOINT")
	if !cfg.Endpoint.IsNull() && cfg.Endpoint.ValueString() != "" {
		baseURL = cfg.Endpoint.ValueString()
	}
	if baseURL == "" {
		baseURL = client.DefaultBaseURL
	}
	if token == "" {
		resp.Diagnostics.AddError("Missing Privatus Analytics API token",
			"Set `api_token` in the provider block or the PRIVATUS_API_TOKEN environment variable. "+
				"Create a token under Settings → API tokens.")
		return
	}

	c := client.New(baseURL, token, "terraform-provider-privatus/"+p.version)
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *privatusProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSiteResource,
		NewGoalResource,
		NewTrafficRuleResource,
	}
}

func (p *privatusProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewWorkspaceDataSource,
	}
}
