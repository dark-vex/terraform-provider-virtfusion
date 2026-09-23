package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	stdpath "path"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
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
	ID types.String `tfsdk:"id"`

	// Create-only inputs. Optional+Computed (not Required) so an imported
	// resource never carries a Required/RequiresReplace attribute the real
	// API can't give back — that combination would make an imported
	// resource's very first plan propose a destroy+recreate. Create()
	// itself still validates these are set.
	ResourcePackID   types.Int64  `tfsdk:"resource_pack_id"`
	CreateID         types.String `tfsdk:"create_id"`
	OverrideMemoryMB types.Int64  `tfsdk:"override_memory_mb"`
	OverrideStorage  types.Int64  `tfsdk:"override_storage_gb"`
	OverrideCPUCores types.Int64  `tfsdk:"override_cpu_cores"`

	// Updatable via their own narrow endpoints.
	Name              types.String `tfsdk:"name"`
	BootOrder         types.String `tfsdk:"boot_order"`
	BootType          types.String `tfsdk:"boot_type"`
	AutoConfiguration types.Bool   `tfsdk:"auto_configuration"`

	// Read-only, mapped from the real server object.
	Hostname       types.String `tfsdk:"hostname"`
	Suspended      types.Bool   `tfsdk:"suspended"`
	Protected      types.Bool   `tfsdk:"protected"`
	Migrating      types.Bool   `tfsdk:"migrating"`
	Deleting       types.Bool   `tfsdk:"deleting"`
	BackupCreating types.Bool   `tfsdk:"backup_creating"`
	Rescue         types.Bool   `tfsdk:"rescue"`
	VNCEnabled     types.Bool   `tfsdk:"vnc_enabled"`
	ISOMounted     types.Bool   `tfsdk:"iso_mounted"`
	UEFI           types.Bool   `tfsdk:"uefi"`
	Memory         types.String `tfsdk:"memory"`
	CPU            types.String `tfsdk:"cpu"`

	Storage []VirtfusionServerStorageModel `tfsdk:"storage"`
	Network *VirtfusionServerNetworkModel  `tfsdk:"network"`

	CurrentMonthlyPeriod *VirtfusionServerPeriodModel `tfsdk:"current_monthly_period"`
	Created              types.String                 `tfsdk:"created"`
	State                types.String                 `tfsdk:"state"`
}

type VirtfusionServerStorageModel struct {
	Capacity types.String `tfsdk:"capacity"`
	Enabled  types.Bool   `tfsdk:"enabled"`
	Primary  types.Bool   `tfsdk:"primary"`
	Created  types.String `tfsdk:"created"`
}

type VirtfusionServerNetworkModel struct {
	Primary   *VirtfusionNetworkInterfaceModel  `tfsdk:"primary"`
	Secondary []VirtfusionNetworkInterfaceModel `tfsdk:"secondary"`
}

type VirtfusionNetworkInterfaceModel struct {
	MAC   types.String          `tfsdk:"mac"`
	Limit types.String          `tfsdk:"limit"`
	IPv4  []VirtfusionIPv4Model `tfsdk:"ipv4"`
	IPv6  []VirtfusionIPv6Model `tfsdk:"ipv6"`
}

type VirtfusionIPv4Model struct {
	Address types.String `tfsdk:"address"`
	Gateway types.String `tfsdk:"gateway"`
	Netmask types.String `tfsdk:"netmask"`
}

type VirtfusionIPv6Model struct {
	Subnet    types.String `tfsdk:"subnet"`
	Gateway   types.String `tfsdk:"gateway"`
	Addresses types.List   `tfsdk:"addresses"`
}

type VirtfusionServerPeriodModel struct {
	Start types.String `tfsdk:"start"`
	End   types.String `tfsdk:"end"`
}

func (r *VirtfusionServerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "virtfusion_server"
}

func networkInterfaceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"mac": schema.StringAttribute{
			Computed: true,
		},
		"limit": schema.StringAttribute{
			Computed: true,
		},
		"ipv4": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"address": schema.StringAttribute{Computed: true},
					"gateway": schema.StringAttribute{Computed: true},
					"netmask": schema.StringAttribute{Computed: true},
				},
			},
		},
		"ipv6": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"subnet":  schema.StringAttribute{Computed: true},
					"gateway": schema.StringAttribute{Computed: true},
					"addresses": schema.ListAttribute{
						Computed:    true,
						ElementType: types.StringType,
					},
				},
			},
		},
	}
}

func (r *VirtfusionServerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Represents a VirtFusion server. Server creation goes through a resource-pack " +
			"discovery flow (see `resource_pack_id`/`create_id`) rather than a flat parameter list. Most " +
			"attributes have no update endpoint on the real API and are `RequiresReplace`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Server UUID.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"resource_pack_id": schema.Int64Attribute{
				MarkdownDescription: "Resource pack ID to create the server from (see `GET /resourcePack`). " +
					"Optional+Computed rather than Required so an imported server never shows a forced " +
					"replace — but it (and `create_id`) must be set in config for `Create` to succeed.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"create_id": schema.StringAttribute{
				MarkdownDescription: "Opaque create-option ID within the resource pack (see " +
					"`GET /resourcePack/{resourcePackId}`), required for Create.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"override_memory_mb": schema.Int64Attribute{
				MarkdownDescription: "Memory override in MB. Only used (and only meaningful) for resource " +
					"pack options with a variable size; ignored for fixed-size packs.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"override_storage_gb": schema.Int64Attribute{
				MarkdownDescription: "Storage override in GB. Only used for variable resource pack options.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"override_cpu_cores": schema.Int64Attribute{
				MarkdownDescription: "CPU core count override. Only used for variable resource pack options.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Confirmed live: the real create endpoint has no name field, and a " +
					"freshly created (not yet built) server rejects renames with 409. Omit this on the apply " +
					"that creates the server; set it on a later apply after building it with " +
					"virtfusion_server_build.",
				Optional: true,
				Computed: true,
			},
			"boot_order": schema.StringAttribute{
				MarkdownDescription: "One of \"hdd,cdrom\" or \"cdrom,hdd\".",
				Optional:            true,
				Computed:            true,
			},
			"boot_type": schema.StringAttribute{
				MarkdownDescription: "One of \"uefi\" or \"bios\". Write-only: the real API does not return " +
					"this on read, so it is not refreshed from the server — only what you last applied.",
				Optional: true,
			},
			"auto_configuration": schema.BoolAttribute{
				MarkdownDescription: "Write-only, see `boot_type`.",
				Optional:            true,
			},
			"hostname": schema.StringAttribute{
				Computed: true,
			},
			"suspended":       schema.BoolAttribute{Computed: true},
			"protected":       schema.BoolAttribute{Computed: true},
			"migrating":       schema.BoolAttribute{Computed: true},
			"deleting":        schema.BoolAttribute{Computed: true},
			"backup_creating": schema.BoolAttribute{Computed: true},
			"rescue":          schema.BoolAttribute{Computed: true},
			"vnc_enabled":     schema.BoolAttribute{Computed: true},
			"iso_mounted":     schema.BoolAttribute{Computed: true},
			"uefi":            schema.BoolAttribute{Computed: true},
			"memory": schema.StringAttribute{
				MarkdownDescription: "Raw memory string as returned by the API (e.g. \"10240 MB\").",
				Computed:            true,
			},
			"cpu": schema.StringAttribute{
				MarkdownDescription: "Raw CPU string as returned by the API (e.g. \"2 Core\").",
				Computed:            true,
			},
			"storage": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"capacity": schema.StringAttribute{Computed: true},
						"enabled":  schema.BoolAttribute{Computed: true},
						"primary":  schema.BoolAttribute{Computed: true},
						"created":  schema.StringAttribute{Computed: true},
					},
				},
			},
			"network": schema.SingleNestedAttribute{
				MarkdownDescription: "Carries the prior state value forward during Update (`UseStateForUnknown`): " +
					"the Go model uses a pointer struct for this object, which — unlike `types.Object` — cannot " +
					"represent an Unknown value, so it must never be left Unknown in a plan Update() decodes.",
				Computed: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"primary": schema.SingleNestedAttribute{
						Computed:   true,
						Attributes: networkInterfaceAttributes(),
						PlanModifiers: []planmodifier.Object{
							objectplanmodifier.UseStateForUnknown(),
						},
					},
					"secondary": schema.ListNestedAttribute{
						Computed: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: networkInterfaceAttributes(),
						},
					},
				},
			},
			"current_monthly_period": schema.SingleNestedAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"start": schema.StringAttribute{Computed: true},
					"end":   schema.StringAttribute{Computed: true},
				},
			},
			"created": schema.StringAttribute{
				Computed: true,
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Unconfirmed semantics; observed as null in testing. Passed through as-is.",
				Computed:            true,
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
	// Deliberately not req.Plan.Get(ctx, &data): on Create there is no prior
	// state, so every Computed-only attribute (network,
	// current_monthly_period, ...) is Unknown in the plan, and the pointer
	// struct types used for nested objects (e.g. *VirtfusionServerNetworkModel)
	// cannot represent Unknown — only Null. Pull just the plan-supplied
	// inputs Create actually needs instead; everything else is populated
	// from the real server object via applyServerToModel below.
	var data VirtfusionServerResourceModel
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("resource_pack_id"), &data.ResourcePackID)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("create_id"), &data.CreateID)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("override_memory_mb"), &data.OverrideMemoryMB)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("override_storage_gb"), &data.OverrideStorage)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("override_cpu_cores"), &data.OverrideCPUCores)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("boot_type"), &data.BootType)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("auto_configuration"), &data.AutoConfiguration)...)
	var planName types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("name"), &planName)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// These three are Optional+Computed ("only used for variable resource
	// pack options"): if the user didn't set one, its plan value is Unknown
	// rather than Null (nothing forces it known), but neither the create
	// payload nor applyServerToModel ever populates it afterward — the
	// physical result lives in the separate `memory`/`storage`/`cpu`
	// fields instead. Left as Unknown, State.Set would fail apply's
	// all-values-must-be-known check. Since an unused override has no
	// meaningful value, Null is the correct final state, not Unknown.
	if data.OverrideMemoryMB.IsUnknown() {
		data.OverrideMemoryMB = types.Int64Null()
	}
	if data.OverrideStorage.IsUnknown() {
		data.OverrideStorage = types.Int64Null()
	}
	if data.OverrideCPUCores.IsUnknown() {
		data.OverrideCPUCores = types.Int64Null()
	}

	if data.ResourcePackID.IsNull() || data.CreateID.IsNull() {
		resp.Diagnostics.AddError(
			"Missing resource_pack_id/create_id",
			"resource_pack_id and create_id are Optional+Computed (so an imported server never forces a "+
				"replace), but both are required to create a new server. Look them up via GET /resourcePack "+
				"and GET /resourcePack/{resourcePackId} against this account, then set them explicitly.",
		)
		return
	}

	payload := map[string]interface{}{}
	if !data.OverrideMemoryMB.IsNull() {
		payload["memory"] = data.OverrideMemoryMB.ValueInt64()
	}
	if !data.OverrideStorage.IsNull() {
		payload["storage"] = data.OverrideStorage.ValueInt64()
	}
	if !data.OverrideCPUCores.IsNull() {
		payload["cpuCores"] = data.OverrideCPUCores.ValueInt64()
	}

	var body []byte
	if len(payload) > 0 {
		body, _ = json.Marshal(payload)
	}

	relPath := stdpath.Join("/resourcePack", strconv.FormatInt(data.ResourcePackID.ValueInt64(), 10), data.CreateID.ValueString())
	httpReq, err := newAPIRequest(ctx, r.config, "POST", relPath, body)
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

	var created APICreateServerResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&created); err != nil {
		resp.Diagnostics.AddError("Error decoding API response", err.Error())
		return
	}
	if created.ID == "" {
		resp.Diagnostics.AddError("Unexpected API Response", "Create response did not include an id.")
		return
	}

	server, diags := r.fetchServer(ctx, created.ID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if server == nil {
		resp.Diagnostics.AddError("Server not found after creation", fmt.Sprintf("Created server %s but a follow-up GET returned 404.", created.ID))
		return
	}

	applyServerToModel(*server, &data)

	// The create endpoint has no name field (confirmed against the real API
	// spec: only memory/storage/cpuCores), so a server always starts out
	// with whatever default name the panel assigns. Reconcile immediately if
	// the config requested a specific name, otherwise the plan's known
	// `name` value would mismatch the applied state.
	//
	// Confirmed live: a just-created server (before it has been built via
	// virtfusion_server_build) rejects PUT .../name with 409 "server is not
	// in a valid state" — not transient, still 409 after a 5s retry. If that
	// happens, surface a clear explanation instead of either a cryptic
	// Terraform-core "inconsistent result" error or a bare "409" message.
	if !planName.IsNull() && !planName.IsUnknown() && !planName.Equal(data.Name) {
		body, _ := json.Marshal(map[string]interface{}{"name": planName.ValueString()})
		httpReq, err := newAPIRequest(ctx, r.config, "PUT", stdpath.Join(apiServerPath(data.ID.ValueString()), "name"), body)
		if err != nil {
			resp.Diagnostics.AddError("Error creating request", err.Error())
			return
		}
		httpResp, err := r.client.Do(httpReq)
		if err != nil {
			resp.Diagnostics.AddError("API request failed", err.Error())
			return
		}
		httpResp.Body.Close()
		if httpResp.StatusCode == http.StatusConflict {
			resp.Diagnostics.AddError(
				"Cannot set name at creation time",
				fmt.Sprintf(
					"Server %s was created successfully, but the real API refused to set its name (409: not in "+
						"a valid state) because it has not been built yet. Omit `name` on the apply that creates "+
						"this server, build it with virtfusion_server_build, then set `name` on a later apply.",
					data.ID.ValueString(),
				),
			)
			return
		}
		if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusNoContent {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("PUT .../name status: %d", httpResp.StatusCode))
			return
		}
		data.Name = planName
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VirtfusionServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VirtfusionServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	server, diags := r.fetchServer(ctx, data.ID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if server == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	applyServerToModel(*server, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VirtfusionServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state VirtfusionServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()

	if !plan.Name.Equal(state.Name) && !plan.Name.IsUnknown() {
		body, _ := json.Marshal(map[string]interface{}{"name": plan.Name.ValueString()})
		httpReq, err := newAPIRequest(ctx, r.config, "PUT", stdpath.Join(apiServerPath(id), "name"), body)
		if err != nil {
			resp.Diagnostics.AddError("Error creating request", err.Error())
			return
		}
		httpResp, err := r.client.Do(httpReq)
		if err != nil {
			resp.Diagnostics.AddError("API request failed", err.Error())
			return
		}
		httpResp.Body.Close()
		if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusNoContent {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("PUT .../name status: %d", httpResp.StatusCode))
			return
		}
	}

	if (!plan.BootType.Equal(state.BootType) && !plan.BootType.IsUnknown()) ||
		(!plan.AutoConfiguration.Equal(state.AutoConfiguration) && !plan.AutoConfiguration.IsUnknown()) {
		settings := map[string]interface{}{}
		if !plan.BootType.IsNull() {
			settings["bootType"] = plan.BootType.ValueString()
		}
		if !plan.AutoConfiguration.IsNull() {
			settings["autoConfiguration"] = plan.AutoConfiguration.ValueBool()
		}
		body, _ := json.Marshal(settings)
		httpReq, err := newAPIRequest(ctx, r.config, "PUT", stdpath.Join(apiServerPath(id), "settings"), body)
		if err != nil {
			resp.Diagnostics.AddError("Error creating request", err.Error())
			return
		}
		httpResp, err := r.client.Do(httpReq)
		if err != nil {
			resp.Diagnostics.AddError("API request failed", err.Error())
			return
		}
		httpResp.Body.Close()
		if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusNoContent {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("PUT .../settings status: %d", httpResp.StatusCode))
			return
		}
	}

	if !plan.BootOrder.Equal(state.BootOrder) && !plan.BootOrder.IsUnknown() && !plan.BootOrder.IsNull() {
		order := plan.BootOrder.ValueString()
		if order != "hdd,cdrom" && order != "cdrom,hdd" {
			resp.Diagnostics.AddError("Invalid boot_order", `boot_order must be exactly "hdd,cdrom" or "cdrom,hdd".`)
			return
		}
		body, _ := json.Marshal(map[string]interface{}{"order": order})
		httpReq, err := newAPIRequest(ctx, r.config, "POST", stdpath.Join(apiServerPath(id), "bootOrder"), body)
		if err != nil {
			resp.Diagnostics.AddError("Error creating request", err.Error())
			return
		}
		httpResp, err := r.client.Do(httpReq)
		if err != nil {
			resp.Diagnostics.AddError("API request failed", err.Error())
			return
		}
		var taskEnv APITaskEnvelope
		decodeErr := json.NewDecoder(httpResp.Body).Decode(&taskEnv)
		httpResp.Body.Close()
		if httpResp.StatusCode != http.StatusOK {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("POST .../bootOrder status: %d", httpResp.StatusCode))
			return
		}
		if decodeErr == nil && taskEnv.Data.Task.ID != 0 {
			if _, err := pollTask(ctx, r.client, r.config, id, taskEnv.Data.Task.ID); err != nil {
				resp.Diagnostics.AddError("boot order task did not complete", err.Error())
				return
			}
		}
	}

	server, diags := r.fetchServer(ctx, id)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if server == nil {
		resp.Diagnostics.AddError("Server not found after update", fmt.Sprintf("Server %s no longer exists.", id))
		return
	}

	applyServerToModel(*server, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VirtfusionServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VirtfusionServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	relPath := stdpath.Join("/resourcePack", data.ID.ValueString())
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
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Status: %d. Note: DELETE /resourcePack/{serverId} only works for servers created "+
				"as part of a resource pack, per the API's own documentation — a server that predates "+
				"resource-pack provisioning (e.g. one brought in via terraform import) may not be deletable "+
				"through this endpoint at all.", httpResp.StatusCode),
		)
		return
	}
}

// fetchServer GETs /server/{id} and returns (nil, nil) on a 404, so callers
// can distinguish "gone" from an actual error.
func (r *VirtfusionServerResource) fetchServer(ctx context.Context, id string) (*APIServer, diag.Diagnostics) {
	var diags diag.Diagnostics

	httpReq, err := newAPIRequest(ctx, r.config, "GET", apiServerPath(id), nil)
	if err != nil {
		diags.AddError("Error creating request", err.Error())
		return nil, diags
	}

	httpResp, err := r.client.Do(httpReq)
	if err != nil {
		diags.AddError("API request failed", err.Error())
		return nil, diags
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if httpResp.StatusCode != http.StatusOK {
		diags.AddError("Unexpected API Response", fmt.Sprintf("status: %d", httpResp.StatusCode))
		return nil, diags
	}

	var envelope APIServerDetailEnvelope
	if err := json.NewDecoder(httpResp.Body).Decode(&envelope); err != nil {
		diags.AddError("Error decoding API response", err.Error())
		return nil, diags
	}
	if envelope.Data.ID == "" {
		return nil, nil
	}

	return &envelope.Data, nil
}

// applyServerToModel overwrites every field the real API can populate,
// while leaving create-only/write-only inputs (resource_pack_id, create_id,
// override_*, boot_type, auto_configuration) exactly as they already are in
// model, since the API never returns them.
func applyServerToModel(s APIServer, model *VirtfusionServerResourceModel) {
	model.ID = types.StringValue(s.ID)
	model.Name = types.StringValue(s.Name)

	if s.Hostname != nil {
		model.Hostname = types.StringValue(*s.Hostname)
	} else {
		model.Hostname = types.StringNull()
	}

	model.Suspended = types.BoolValue(s.Suspended)
	model.Protected = types.BoolValue(s.Protected)
	model.Migrating = types.BoolValue(s.Migrating)
	model.Deleting = types.BoolValue(s.Deleting)
	model.BackupCreating = types.BoolValue(s.BackupCreating)
	model.Rescue = types.BoolValue(s.Rescue)
	model.VNCEnabled = types.BoolValue(s.VNCEnabled)
	model.ISOMounted = types.BoolValue(s.ISOMounted)
	model.UEFI = types.BoolValue(s.UEFI)

	model.BootOrder = types.StringValue(normalizeBootOrder(s.BootOrder))

	model.Memory = types.StringValue(s.Memory)
	model.CPU = types.StringValue(s.CPU)

	storage := make([]VirtfusionServerStorageModel, 0, len(s.Storage))
	for _, item := range s.Storage {
		storage = append(storage, VirtfusionServerStorageModel{
			Capacity: types.StringValue(item.Capacity),
			Enabled:  types.BoolValue(item.Enabled),
			Primary:  types.BoolValue(item.Primary),
			Created:  types.StringValue(item.Created),
		})
	}
	model.Storage = storage

	secondary := make([]VirtfusionNetworkInterfaceModel, 0, len(s.Network.Secondary))
	for _, iface := range s.Network.Secondary {
		secondary = append(secondary, networkInterfaceToModel(iface))
	}
	primary := networkInterfaceToModel(s.Network.Primary)
	model.Network = &VirtfusionServerNetworkModel{
		Primary:   &primary,
		Secondary: secondary,
	}

	model.CurrentMonthlyPeriod = &VirtfusionServerPeriodModel{
		Start: types.StringValue(s.CurrentMonthlyPeriod.Start),
		End:   types.StringValue(s.CurrentMonthlyPeriod.End),
	}
	model.Created = types.StringValue(s.Created)
	if s.State != nil {
		model.State = types.StringValue(*s.State)
	} else {
		model.State = types.StringNull()
	}
}

// normalizeBootOrder maps the real Read shape (e.g. ["hd","cdrom"]) onto the
// single comma-joined enum string the write endpoint expects
// ("hdd,cdrom"/"cdrom,hdd"), best-effort. Falls back to a raw join if the
// shape doesn't match either known 2-element form.
func normalizeBootOrder(order []string) string {
	norm := make([]string, 0, len(order))
	for _, o := range order {
		switch strings.ToLower(o) {
		case "hd", "hdd":
			norm = append(norm, "hdd")
		case "cdrom", "cd":
			norm = append(norm, "cdrom")
		default:
			norm = append(norm, o)
		}
	}
	return strings.Join(norm, ",")
}

func networkInterfaceToModel(iface APINetworkInterface) VirtfusionNetworkInterfaceModel {
	ipv4 := make([]VirtfusionIPv4Model, 0, len(iface.IPv4))
	for _, a := range iface.IPv4 {
		ipv4 = append(ipv4, VirtfusionIPv4Model{
			Address: types.StringValue(a.Address),
			Gateway: types.StringValue(a.Gateway),
			Netmask: types.StringValue(a.Netmask),
		})
	}

	ipv6 := make([]VirtfusionIPv6Model, 0, len(iface.IPv6))
	for _, a := range iface.IPv6 {
		addresses := make([]types.String, 0, len(a.Addresses))
		for _, addr := range a.Addresses {
			addresses = append(addresses, types.StringValue(addr))
		}
		addressesList, _ := types.ListValueFrom(context.Background(), types.StringType, addresses)
		ipv6 = append(ipv6, VirtfusionIPv6Model{
			Subnet:    types.StringValue(a.Subnet),
			Gateway:   types.StringValue(a.Gateway),
			Addresses: addressesList,
		})
	}

	return VirtfusionNetworkInterfaceModel{
		MAC:   types.StringValue(iface.MAC),
		Limit: types.StringValue(iface.Limit),
		IPv4:  ipv4,
		IPv6:  ipv6,
	}
}
