// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/http"
	"net/url"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure VirtfusionProvider satisfies provider.Provider interface
var _ provider.Provider = &VirtfusionProvider{}

// VirtfusionProvider implements the provider.
type VirtfusionProvider struct {
	version string
}

// ProviderConfig is shared with resources and data sources.
type ProviderConfig struct {
	Client   *http.Client
	Endpoint string
	// BaseURL is the single source of truth for composing request URLs
	// (scheme + host + base API path). All resource code must build
	// requests through newAPIRequest using a relative path against this
	// base — never by string-concatenating Endpoint directly.
	BaseURL  *url.URL
	ApiToken string
}

// VirtfusionProviderModel describes the provider schema.
type VirtfusionProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	ApiToken types.String `tfsdk:"api_token"`
}

func (p *VirtfusionProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "virtfusion"
	resp.Version = p.version
}

func (p *VirtfusionProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "VirtFusion API host (bare host, no scheme), e.g. \"vps.hostbrr.com\". Required; there is no default since this fork targets one specific deployment.",
				Optional:            true,
			},
			"api_token": schema.StringAttribute{
				MarkdownDescription: "API token for authentication.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *VirtfusionProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data VirtfusionProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Environment defaults
	apiToken := os.Getenv("VIRTFUSION_API_TOKEN")
	endpoint := os.Getenv("VIRTFUSION_ENDPOINT")

	// Override from config
	if !data.Endpoint.IsNull() {
		endpoint = data.Endpoint.ValueString()
	}
	if endpoint == "" {
		resp.Diagnostics.AddError(
			"Missing Endpoint",
			"No API endpoint provided via config (endpoint) or VIRTFUSION_ENDPOINT env var. This fork does not default to any panel host.",
		)
		return
	}

	if !data.ApiToken.IsNull() {
		apiToken = data.ApiToken.ValueString()
	}
	if apiToken == "" {
		resp.Diagnostics.AddError(
			"Missing API Token",
			"No API token provided via config or VIRTFUSION_API_TOKEN env var.",
		)
		return
	}

	baseURL := &url.URL{Scheme: "https", Host: endpoint, Path: "/api"}

	// Build HTTP client
	customTransport := &CustomTransport{
		Transport: http.DefaultTransport,
		Token:     apiToken,
	}
	client := &http.Client{Transport: customTransport}

	// Share provider config with resources
	config := &ProviderConfig{
		Client:   client,
		Endpoint: endpoint,
		BaseURL:  baseURL,
		ApiToken: apiToken,
	}

	resp.DataSourceData = config
	resp.ResourceData = config
}

func (p *VirtfusionProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewVirtfusionServerResource,
		NewVirtfusionServerBuildResource,
		NewVirtfusionSSHResource,
	}
}

func (p *VirtfusionProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

// CustomTransport applies authentication only. URL composition is handled
// centrally by newAPIRequest (see client.go) — this transport must never
// mutate req.URL, since doing so on top of an already-fully-qualified
// request is what previously double-embedded the host into the path.
type CustomTransport struct {
	Transport http.RoundTripper
	Token     string
}

func (c *CustomTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+c.Token)
	return c.Transport.RoundTrip(req)
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &VirtfusionProvider{version: version}
	}
}
