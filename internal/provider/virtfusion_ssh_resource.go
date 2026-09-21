package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	stdpath "path"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
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

// VirtfusionSSHResourceModel has no user_id: the real API scopes SSH keys
// to the account via the bearer token, not a request field. There is no
// update endpoint on the real API either — name/public_key are
// RequiresReplace.
type VirtfusionSSHResourceModel struct {
	ID        types.Int64  `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	PublicKey types.String `tfsdk:"public_key"`
}

func (r *VirtfusionSSHResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "virtfusion_ssh"
}

func (r *VirtfusionSSHResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Represents a VirtFusion account SSH key. The real API has no update endpoint " +
			"for SSH keys, so `name`/`public_key` are `RequiresReplace`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"public_key": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
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
		"name":      data.Name.ValueString(),
		"publicKey": data.PublicKey.ValueString(),
	}

	body, _ := json.Marshal(payload)

	httpReq, err := newAPIRequest(ctx, r.config, "POST", "/account/sshKeys", body)
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

	// The create response is the same paginated list envelope as a plain
	// list (not the single created object) — find our new key by matching
	// the public key we just submitted.
	var envelope APISSHKeyListEnvelope
	if err := json.NewDecoder(httpResp.Body).Decode(&envelope); err != nil {
		resp.Diagnostics.AddError("Error decoding API response", err.Error())
		return
	}

	key, found := findSSHKeyByPublicKey(envelope.Data, data.PublicKey.ValueString())
	if !found {
		resp.Diagnostics.AddError(
			"SSH key not found after creation",
			"The create response's key list did not contain a key matching the submitted public_key.",
		)
		return
	}

	data.ID = types.Int64Value(key.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read has no GET-by-id to call — the real API only exposes a paginated
// list, so this lists (following pagination if needed) and filters by id.
func (r *VirtfusionSSHResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VirtfusionSSHResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, found, diags := r.findSSHKeyByID(ctx, data.ID.ValueInt64())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Name = types.StringValue(key.Name)
	data.PublicKey = types.StringValue(key.PublicKey)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update is unreachable: both attributes are RequiresReplace (the real API
// has no update endpoint), so Terraform always does Delete+Create instead.
func (r *VirtfusionSSHResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"virtfusion_ssh has no Update",
		"Both attributes are RequiresReplace; Terraform should never call Update for this resource.",
	)
}

func (r *VirtfusionSSHResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VirtfusionSSHResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	relPath := stdpath.Join("/account/sshKeys", strconv.FormatInt(data.ID.ValueInt64(), 10))
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

func findSSHKeyByPublicKey(keys []APISSHKey, publicKey string) (APISSHKey, bool) {
	for _, k := range keys {
		if k.PublicKey == publicKey {
			return k, true
		}
	}
	return APISSHKey{}, false
}

// findSSHKeyByID walks GET /account/sshKeys, following next_page_url, until
// it finds a key with the given id or runs out of pages.
func (r *VirtfusionSSHResource) findSSHKeyByID(ctx context.Context, id int64) (APISSHKey, bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	httpReq, err := newAPIRequest(ctx, r.config, "GET", "/account/sshKeys", nil)
	if err != nil {
		diags.AddError("Error creating request", err.Error())
		return APISSHKey{}, false, diags
	}

	for {
		httpResp, err := r.client.Do(httpReq)
		if err != nil {
			diags.AddError("API request failed", err.Error())
			return APISSHKey{}, false, diags
		}

		var envelope APISSHKeyListEnvelope
		decodeErr := json.NewDecoder(httpResp.Body).Decode(&envelope)
		status := httpResp.StatusCode
		httpResp.Body.Close()

		if status != http.StatusOK {
			diags.AddError("Unexpected API Response", fmt.Sprintf("unexpected status %d listing SSH keys", status))
			return APISSHKey{}, false, diags
		}
		if decodeErr != nil {
			diags.AddError("Error decoding API response", decodeErr.Error())
			return APISSHKey{}, false, diags
		}

		for _, k := range envelope.Data {
			if k.ID == id {
				return k, true, nil
			}
		}

		if envelope.NextPageURL == nil {
			return APISSHKey{}, false, nil
		}

		httpReq, err = newAPIRequestAbsolute(ctx, "GET", *envelope.NextPageURL, nil)
		if err != nil {
			diags.AddError("Error creating request", err.Error())
			return APISSHKey{}, false, diags
		}
	}
}
