package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Hostname types.String `tfsdk:"hostname"`

	Suspended      types.Bool `tfsdk:"suspended"`
	Protected      types.Bool `tfsdk:"protected"`
	Migrating      types.Bool `tfsdk:"migrating"`
	Deleting       types.Bool `tfsdk:"deleting"`
	BackupCreating types.Bool `tfsdk:"backup_creating"`
	Rescue         types.Bool `tfsdk:"rescue"`
	VNCEnabled     types.Bool `tfsdk:"vnc_enabled"`
	ISOMounted     types.Bool `tfsdk:"iso_mounted"`
	UEFI           types.Bool `tfsdk:"uefi"`

	BootOrder []string `tfsdk:"boot_order"`

	Memory   string `tfsdk:"memory"`
	MemoryMB *int64 `tfsdk:"memory_mb"`
	CPU      string `tfsdk:"cpu"`
	CPUCores *int64 `tfsdk:"cpu_cores"`

	Storage []VirtfusionServerStorageModel `tfsdk:"storage"`
	Network VirtfusionServerNetworkModel   `tfsdk:"network"`

	CurrentMonthlyPeriod VirtfusionServerPeriodModel `tfsdk:"current_monthly_period"`
	Created              string                      `tfsdk:"created"`
	State                *string                     `tfsdk:"state"`
}

type VirtfusionServerStorageModel struct {
	Capacity   string `tfsdk:"capacity"`
	CapacityGB *int64 `tfsdk:"capacity_gb"`
	Enabled    bool   `tfsdk:"enabled"`
	Primary    bool   `tfsdk:"primary"`
	Created    string `tfsdk:"created"`
}

type VirtfusionServerNetworkModel struct {
	Primary   VirtfusionNetworkInterfaceModel   `tfsdk:"primary"`
	Secondary []VirtfusionNetworkInterfaceModel `tfsdk:"secondary"`
}

type VirtfusionNetworkInterfaceModel struct {
	MAC   string                `tfsdk:"mac"`
	Limit string                `tfsdk:"limit"`
	IPv4  []VirtfusionIPv4Model `tfsdk:"ipv4"`
	IPv6  []VirtfusionIPv6Model `tfsdk:"ipv6"`
}

type VirtfusionIPv4Model struct {
	Address string `tfsdk:"address"`
	Gateway string `tfsdk:"gateway"`
	Netmask string `tfsdk:"netmask"`
}

type VirtfusionIPv6Model struct {
	Subnet    string   `tfsdk:"subnet"`
	Gateway   string   `tfsdk:"gateway"`
	Addresses []string `tfsdk:"addresses"`
}

type VirtfusionServerPeriodModel struct {
	Start string `tfsdk:"start"`
	End   string `tfsdk:"end"`
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
		MarkdownDescription: "Represents a VirtFusion server. Create/Update/Delete are not implemented against this " +
			"fork's real API shape (unverified — see CODE-27); this resource is intended to be brought under " +
			"management exclusively via `terraform import`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Server UUID.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"hostname": schema.StringAttribute{
				Optional: true,
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
			"boot_order": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
			"memory": schema.StringAttribute{
				MarkdownDescription: "Raw memory string as returned by the API (e.g. \"10240 MB\").",
				Computed:            true,
			},
			"memory_mb": schema.Int64Attribute{
				MarkdownDescription: "Best-effort parsed memory in MB. Null if `memory` could not be parsed.",
				Computed:            true,
			},
			"cpu": schema.StringAttribute{
				MarkdownDescription: "Raw CPU string as returned by the API (e.g. \"2 Core\").",
				Computed:            true,
			},
			"cpu_cores": schema.Int64Attribute{
				MarkdownDescription: "Best-effort parsed CPU core count. Null if `cpu` could not be parsed.",
				Computed:            true,
			},
			"storage": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"capacity": schema.StringAttribute{Computed: true},
						"capacity_gb": schema.Int64Attribute{
							MarkdownDescription: "Best-effort parsed capacity in GB. Null if `capacity` could not be parsed.",
							Computed:            true,
						},
						"enabled": schema.BoolAttribute{Computed: true},
						"primary": schema.BoolAttribute{Computed: true},
						"created": schema.StringAttribute{Computed: true},
					},
				},
			},
			"network": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"primary": schema.SingleNestedAttribute{
						Computed:   true,
						Attributes: networkInterfaceAttributes(),
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

func (r *VirtfusionServerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

const unverifiedMutationSummary = "Not verified against this VirtFusion deployment"

const unverifiedMutationDetail = "The request/response shape for this operation has not been confirmed against the " +
	"real VirtFusion API this fork targets (see CODE-27). Sending a guessed payload could mutate a live production " +
	"server, so this is deliberately not implemented. Bring existing servers under management with `terraform " +
	"import` instead."

func (r *VirtfusionServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(unverifiedMutationSummary, unverifiedMutationDetail)
}

func (r *VirtfusionServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Only the id is read out of prior state here, deliberately. Right after
	// ImportStatePassthroughID, every other attribute in state is still
	// null — decoding the full model (most fields use non-pointer native Go
	// types) would fail on the first null field it hit.
	var id string
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpReq, err := newAPIRequest(ctx, r.config, "GET", apiServerPath(id), nil)
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

	if httpResp.StatusCode == http.StatusNotFound || httpResp.StatusCode == http.StatusForbidden {
		resp.State.RemoveResource(ctx)
		return
	}
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Status: %d", httpResp.StatusCode))
		return
	}

	var envelope APIServerDetailEnvelope
	if err := json.NewDecoder(httpResp.Body).Decode(&envelope); err != nil {
		resp.Diagnostics.AddError("Error decoding API response", err.Error())
		return
	}

	// A 200 with an empty data object has not been observed but is handled
	// defensively since it is indistinguishable from "gone" at this layer.
	if envelope.Data.ID == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, apiServerToModel(envelope.Data))...)
}

func (r *VirtfusionServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(unverifiedMutationSummary, unverifiedMutationDetail)
}

func (r *VirtfusionServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddError(unverifiedMutationSummary, unverifiedMutationDetail)
}

var leadingIntPattern = regexp.MustCompile(`^\s*(\d+)`)

// parseLeadingInt best-effort parses a leading integer out of strings like
// "10240 MB", "2 Core", "80 GB". Returns nil (not an error) on any mismatch
// so a single unexpected format never fails the whole Read.
func parseLeadingInt(s string) *int64 {
	match := leadingIntPattern.FindStringSubmatch(s)
	if match == nil {
		return nil
	}
	var v int64
	if _, err := fmt.Sscanf(match[1], "%d", &v); err != nil {
		return nil
	}
	return &v
}

func apiServerToModel(s APIServer) *VirtfusionServerResourceModel {
	storage := make([]VirtfusionServerStorageModel, 0, len(s.Storage))
	for _, item := range s.Storage {
		storage = append(storage, VirtfusionServerStorageModel{
			Capacity:   item.Capacity,
			CapacityGB: parseLeadingInt(item.Capacity),
			Enabled:    item.Enabled,
			Primary:    item.Primary,
			Created:    item.Created,
		})
	}

	bootOrder := s.BootOrder
	if bootOrder == nil {
		bootOrder = []string{}
	}

	secondary := make([]VirtfusionNetworkInterfaceModel, 0, len(s.Network.Secondary))
	for _, iface := range s.Network.Secondary {
		secondary = append(secondary, networkInterfaceToModel(iface))
	}

	return &VirtfusionServerResourceModel{
		ID:       types.StringValue(s.ID),
		Name:     types.StringValue(s.Name),
		Hostname: types.StringValue(s.Hostname),

		Suspended:      types.BoolValue(s.Suspended),
		Protected:      types.BoolValue(s.Protected),
		Migrating:      types.BoolValue(s.Migrating),
		Deleting:       types.BoolValue(s.Deleting),
		BackupCreating: types.BoolValue(s.BackupCreating),
		Rescue:         types.BoolValue(s.Rescue),
		VNCEnabled:     types.BoolValue(s.VNCEnabled),
		ISOMounted:     types.BoolValue(s.ISOMounted),
		UEFI:           types.BoolValue(s.UEFI),

		BootOrder: bootOrder,

		Memory:   s.Memory,
		MemoryMB: parseLeadingInt(s.Memory),
		CPU:      s.CPU,
		CPUCores: parseLeadingInt(s.CPU),

		Storage: storage,
		Network: VirtfusionServerNetworkModel{
			Primary:   networkInterfaceToModel(s.Network.Primary),
			Secondary: secondary,
		},

		CurrentMonthlyPeriod: VirtfusionServerPeriodModel{
			Start: s.CurrentMonthlyPeriod.Start,
			End:   s.CurrentMonthlyPeriod.End,
		},
		Created: s.Created,
		State:   s.State,
	}
}

func networkInterfaceToModel(iface APINetworkInterface) VirtfusionNetworkInterfaceModel {
	ipv4 := make([]VirtfusionIPv4Model, 0, len(iface.IPv4))
	for _, a := range iface.IPv4 {
		ipv4 = append(ipv4, VirtfusionIPv4Model{
			Address: a.Address,
			Gateway: a.Gateway,
			Netmask: a.Netmask,
		})
	}

	ipv6 := make([]VirtfusionIPv6Model, 0, len(iface.IPv6))
	for _, a := range iface.IPv6 {
		addresses := a.Addresses
		if addresses == nil {
			addresses = []string{}
		}
		ipv6 = append(ipv6, VirtfusionIPv6Model{
			Subnet:    a.Subnet,
			Gateway:   a.Gateway,
			Addresses: addresses,
		})
	}

	return VirtfusionNetworkInterfaceModel{
		MAC:   iface.MAC,
		Limit: iface.Limit,
		IPv4:  ipv4,
		IPv6:  ipv6,
	}
}
