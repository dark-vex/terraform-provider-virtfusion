package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure implementation
var _ resource.Resource = &VirtfusionServerBuildResource{}

func NewVirtfusionServerBuildResource() resource.Resource {
	return &VirtfusionServerBuildResource{}
}

type VirtfusionServerBuildResource struct {
	client *http.Client
	config *ProviderConfig
}

type VirtfusionServerBuildResourceModel struct {
	ID       types.Int64   `tfsdk:"id"`
	ServerID types.Int64   `tfsdk:"server_id"`
	Name     types.String  `tfsdk:"name"`
	Hostname types.String  `tfsdk:"hostname"`
	OsID     types.Int64   `tfsdk:"osid"`
	VNC      types.Bool    `tfsdk:"vnc"`
	IPv6     types.Bool    `tfsdk:"ipv6"`
	SSHKeys  []types.Int64 `tfsdk:"ssh_keys"`
	Email    types.Bool    `tfsdk:"email"`
}

func (r *VirtfusionServerBuildResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "virtfusion_build"
}

func (r *VirtfusionServerBuildResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Represents a VirtFusion server build. The existence and shape of this API endpoint " +
			"under this fork's deployment is unconfirmed; Create/Update/Delete are not implemented.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
			},
			"server_id": schema.Int64Attribute{
				Required: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"hostname": schema.StringAttribute{
				Required: true,
			},
			"osid": schema.Int64Attribute{
				Optional: true,
			},
			"vnc": schema.BoolAttribute{
				Optional: true,
			},
			"ipv6": schema.BoolAttribute{
				Optional: true,
			},
			"ssh_keys": schema.ListAttribute{
				ElementType: types.Int64Type,
				Optional:    true,
			},
			"email": schema.BoolAttribute{
				Optional: true,
			},
		},
	}
}

func (r *VirtfusionServerBuildResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(*ProviderConfig)
	if !ok || config == nil {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data",
			fmt.Sprintf("Expected *ProviderConfig, got: %T", req.ProviderData),
		)
		return
	}

	r.client = config.Client
	r.config = config
}

func (r *VirtfusionServerBuildResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(unverifiedMutationSummary, unverifiedMutationDetail)
}

func (r *VirtfusionServerBuildResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VirtfusionServerBuildResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Endpoint path unconfirmed for this fork's deployment.
	relPath := "/build/" + strconv.FormatInt(data.ID.ValueInt64(), 10)
	httpReq, err := newAPIRequest(ctx, r.config, "GET", relPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}

	httpResp, err := r.client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("API request failed", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Status: %d", httpResp.StatusCode))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VirtfusionServerBuildResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(unverifiedMutationSummary, unverifiedMutationDetail)
}

func (r *VirtfusionServerBuildResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddError(unverifiedMutationSummary, unverifiedMutationDetail)
}
