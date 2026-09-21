package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure implementation
var (
	_ resource.Resource                = &VirtfusionServerResource{}
	_ resource.ResourceWithImportState = &VirtfusionServerResource{}
)

func NewVirtfusionServerResource() resource.Resource {
	return &VirtfusionServerResource{}
}

type VirtfusionServerResource struct {
	client *http.Client
	config *ProviderConfig
}

type VirtfusionServerResourceModel struct {
	// ID is a UUID string on the real API, not a numeric ID.
	ID               types.String `tfsdk:"id"`
	UserID           types.Int64  `tfsdk:"user_id"`
	PackageID        types.Int64  `tfsdk:"package_id"`
	HypervisorID     types.Int64  `tfsdk:"hypervisor_id"`
	IPv4             types.Int64  `tfsdk:"ipv4"`
	IPv6             types.Int64  `tfsdk:"ipv6"`
	PrivateIPs       types.Int64  `tfsdk:"private_ips"`
	Storage          types.Int64  `tfsdk:"storage"`
	Memory           types.Int64  `tfsdk:"memory"`
	Cores            types.Int64  `tfsdk:"cores"`
	Traffic          types.Int64  `tfsdk:"traffic"`
	InboundSpeed     types.Int64  `tfsdk:"inbound_network_speed"`
	OutboundSpeed    types.Int64  `tfsdk:"outbound_network_speed"`
	StorageProfileID types.Int64  `tfsdk:"storage_profile"`
	NetworkProfileID types.Int64  `tfsdk:"network_profile"`
}

func (r *VirtfusionServerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "virtfusion_server"
}

func (r *VirtfusionServerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Server UUID.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_id": schema.Int64Attribute{
				Required: true,
			},
			"package_id": schema.Int64Attribute{
				Optional: true,
			},
			"hypervisor_id": schema.Int64Attribute{
				Optional: true,
			},
			"ipv4": schema.Int64Attribute{
				Optional: true,
			},
			"ipv6": schema.Int64Attribute{
				Optional: true,
			},
			"private_ips": schema.Int64Attribute{
				Optional: true,
			},
			"storage": schema.Int64Attribute{
				Optional: true,
			},
			"memory": schema.Int64Attribute{
				Optional: true,
			},
			"cores": schema.Int64Attribute{
				Optional: true,
			},
			"traffic": schema.Int64Attribute{
				Optional: true,
			},
			"inbound_network_speed": schema.Int64Attribute{
				Optional: true,
			},
			"outbound_network_speed": schema.Int64Attribute{
				Optional: true,
			},
			"storage_profile": schema.Int64Attribute{
				Optional: true,
			},
			"network_profile": schema.Int64Attribute{
				Optional: true,
			},
		},
	}
}

func (r *VirtfusionServerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState allows an existing server to be brought under management with
// `terraform import virtfusion_server.<name> <uuid>` instead of Create.
func (r *VirtfusionServerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *VirtfusionServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VirtfusionServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Apply provider defaults if values are not set
	if data.PackageID.IsNull() && r.config.ResourcePackage > 0 {
		data.PackageID = types.Int64Value(r.config.ResourcePackage)
	}
	if data.HypervisorID.IsNull() && r.config.HypervisorGroup > 0 {
		data.HypervisorID = types.Int64Value(r.config.HypervisorGroup)
	}
	if data.IPv4.IsNull() && r.config.PublicIPs > 0 {
		data.IPv4 = types.Int64Value(r.config.PublicIPs)
	}
	if data.PrivateIPs.IsNull() && r.config.PrivateIPs > 0 {
		data.PrivateIPs = types.Int64Value(r.config.PrivateIPs)
	}

	// Build request payload
	payload := map[string]interface{}{
		"user_id":                data.UserID.ValueInt64(),
		"package_id":             data.PackageID.ValueInt64(),
		"hypervisor_group_id":    data.HypervisorID.ValueInt64(),
		"ipv4":                   data.IPv4.ValueInt64(),
		"ipv6":                   data.IPv6.ValueInt64(),
		"private_ips":            data.PrivateIPs.ValueInt64(),
		"storage":                data.Storage.ValueInt64(),
		"memory":                 data.Memory.ValueInt64(),
		"cores":                  data.Cores.ValueInt64(),
		"traffic":                data.Traffic.ValueInt64(),
		"inbound_network_speed":  data.InboundSpeed.ValueInt64(),
		"outbound_network_speed": data.OutboundSpeed.ValueInt64(),
		"storage_profile":        data.StorageProfileID.ValueInt64(),
		"network_profile":        data.NetworkProfileID.ValueInt64(),
	}

	body, _ := json.Marshal(payload)

	httpReq, err := newAPIRequest(ctx, r.config, "POST", "/v1/servers", body)
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

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusCreated {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Status: %d", httpResp.StatusCode),
		)
		return
	}

	var respData map[string]interface{}
	if err := json.NewDecoder(httpResp.Body).Decode(&respData); err != nil {
		resp.Diagnostics.AddError("Error decoding API response", err.Error())
		return
	}

	if id, ok := respData["id"].(string); ok {
		data.ID = types.StringValue(id)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VirtfusionServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VirtfusionServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpReq, err := newAPIRequest(ctx, r.config, "GET", apiServerPath(data.ID.ValueString()), nil)
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

	var respData map[string]interface{}
	if err := json.NewDecoder(httpResp.Body).Decode(&respData); err != nil {
		resp.Diagnostics.AddError("Error decoding API response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VirtfusionServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data VirtfusionServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := map[string]interface{}{
		"package_id":  data.PackageID.ValueInt64(),
		"ipv4":        data.IPv4.ValueInt64(),
		"ipv6":        data.IPv6.ValueInt64(),
		"private_ips": data.PrivateIPs.ValueInt64(),
		"storage":     data.Storage.ValueInt64(),
		"memory":      data.Memory.ValueInt64(),
		"cores":       data.Cores.ValueInt64(),
	}

	body, _ := json.Marshal(payload)

	httpReq, err := newAPIRequest(ctx, r.config, "PUT", "/v1/servers/"+data.ID.ValueString(), body)
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

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Status: %d", httpResp.StatusCode))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VirtfusionServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VirtfusionServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpReq, err := newAPIRequest(ctx, r.config, "DELETE", "/v1/servers/"+data.ID.ValueString(), nil)
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

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusNoContent {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Status: %d", httpResp.StatusCode))
		return
	}
}
