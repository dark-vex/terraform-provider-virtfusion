package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure implementation
var _ resource.Resource = &VirtfusionSSHResource{}

func NewVirtfusionSSHResource() resource.Resource {
	return &VirtfusionSSHResource{}
}

type VirtfusionSSHResource struct {
	client *http.Client
	config *ProviderConfig
}

type VirtfusionSSHResourceModel struct {
	ID        types.Int64  `tfsdk:"id"`
	UserID    types.Int64  `tfsdk:"user_id"`
	Name      types.String `tfsdk:"name"`
	PublicKey types.String `tfsdk:"public_key"`
}

func (r *VirtfusionSSHResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "virtfusion_ssh"
}

func (r *VirtfusionSSHResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Represents a VirtFusion SSH key.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
			},
			"user_id": schema.Int64Attribute{
				Required: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"public_key": schema.StringAttribute{
				Required: true,
			},
		},
	}
}

func (r *VirtfusionSSHResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *VirtfusionSSHResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VirtfusionSSHResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := map[string]interface{}{
		"user_id":    data.UserID.ValueInt64(),
		"name":       data.Name.ValueString(),
		"public_key": data.PublicKey.ValueString(),
	}

	body, _ := json.Marshal(payload)

	httpReq, err := newAPIRequest(ctx, r.config, "POST", "/v1/ssh-keys", body)
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

	if id, ok := respData["id"].(float64); ok {
		data.ID = types.Int64Value(int64(id))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VirtfusionSSHResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VirtfusionSSHResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	relPath := "/v1/ssh-keys/" + strconv.FormatInt(data.ID.ValueInt64(), 10)
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

func (r *VirtfusionSSHResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data VirtfusionSSHResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := map[string]interface{}{
		"name":       data.Name.ValueString(),
		"public_key": data.PublicKey.ValueString(),
	}

	body, _ := json.Marshal(payload)

	relPath := "/v1/ssh-keys/" + strconv.FormatInt(data.ID.ValueInt64(), 10)
	httpReq, err := newAPIRequest(ctx, r.config, "PUT", relPath, body)
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

func (r *VirtfusionSSHResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VirtfusionSSHResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	relPath := "/v1/ssh-keys/" + strconv.FormatInt(data.ID.ValueInt64(), 10)
	httpReq, err := newAPIRequest(ctx, r.config, "DELETE", relPath, nil)
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
