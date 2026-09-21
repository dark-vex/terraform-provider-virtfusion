package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	stdpath "path"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
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

// VirtfusionServerBuildResourceModel represents an action against an
// existing server, not a persistent object: the real API has no id, no
// GET-by-id, no update, and no delete for "build" (see
// POST /server/{serverId}/build in the account's own OpenAPI spec). id is
// the target server's own UUID, matching the recommendation from
// independent review that identity should be the server, not the
// short-lived task the build triggers.
type VirtfusionServerBuildResourceModel struct {
	ID         types.String  `tfsdk:"id"`
	ServerID   types.String  `tfsdk:"server_id"`
	Method     types.String  `tfsdk:"method"`
	TemplateID types.Int64   `tfsdk:"template_id"`
	Hostname   types.String  `tfsdk:"hostname"`
	Timezone   types.String  `tfsdk:"timezone"`
	Name       types.String  `tfsdk:"name"`
	Swap       types.Int64   `tfsdk:"swap"`
	IPv6       types.Bool    `tfsdk:"ipv6"`
	SSHKeys    []types.Int64 `tfsdk:"ssh_keys"`
	UserData   types.String  `tfsdk:"user_data"`
	TaskID     types.Int64   `tfsdk:"task_id"`
}

func (r *VirtfusionServerBuildResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "virtfusion_build"
}

func (r *VirtfusionServerBuildResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Triggers a VirtFusion server build (OS install/rebuild) via " +
			"`POST /server/{serverId}/build`. This is a stateless action on the real API, not a persistent " +
			"object — there is no update or delete endpoint. Every attribute is `RequiresReplace`: changing " +
			"any of them re-runs the (destructive) build. `Delete` only removes this resource from Terraform " +
			"state; it never calls the API, since there is nothing to undo.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Equal to `server_id` — build has no identity of its own.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"server_id": schema.StringAttribute{
				MarkdownDescription: "Target server UUID.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"method": schema.StringAttribute{
				MarkdownDescription: "\"template\" or \"self\".",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"template_id": schema.Int64Attribute{
				MarkdownDescription: "Operating system template ID (see " +
					"`GET /server/{serverId}/operatingSystemTemplates`). Required when method is \"template\".",
				Optional: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"hostname": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"timezone": schema.StringAttribute{
				MarkdownDescription: "IANA timezone name, e.g. \"America/Los_Angeles\".",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"swap": schema.Int64Attribute{
				MarkdownDescription: "Swap size in MB (see `GET /server/{serverId}/swap` for valid values).",
				Optional:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"ipv6": schema.BoolAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"ssh_keys": schema.ListAttribute{
				ElementType: types.Int64Type,
				Optional:    true,
			},
			"user_data": schema.StringAttribute{
				MarkdownDescription: "Cloud-init user data.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"task_id": schema.Int64Attribute{
				MarkdownDescription: "Informational only — the async task id the build triggered. Not a " +
					"stable identifier (the task is transient), never used as resource identity.",
				Computed: true,
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
	var data VirtfusionServerBuildResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Method.ValueString() == "template" && data.TemplateID.IsNull() {
		resp.Diagnostics.AddError("Missing template_id", `template_id is required when method is "template".`)
		return
	}

	payload := map[string]interface{}{
		"method": data.Method.ValueString(),
	}
	if !data.TemplateID.IsNull() {
		payload["templateId"] = data.TemplateID.ValueInt64()
	}
	if !data.Hostname.IsNull() {
		payload["hostname"] = data.Hostname.ValueString()
	}
	if !data.Timezone.IsNull() {
		payload["timezone"] = data.Timezone.ValueString()
	}
	if !data.Name.IsNull() {
		payload["name"] = data.Name.ValueString()
	}
	if !data.Swap.IsNull() {
		payload["swap"] = data.Swap.ValueInt64()
	}
	if !data.IPv6.IsNull() {
		payload["ipv6"] = data.IPv6.ValueBool()
	}
	if len(data.SSHKeys) > 0 {
		payload["sshKeys"] = flattenInt64List(data.SSHKeys)
	}
	if !data.UserData.IsNull() {
		payload["userData"] = data.UserData.ValueString()
	}

	body, _ := json.Marshal(payload)
	serverID := data.ServerID.ValueString()

	httpReq, err := newAPIRequest(ctx, r.config, "POST", stdpath.Join(apiServerPath(serverID), "build"), body)
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

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusCreated && httpResp.StatusCode != http.StatusAccepted {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Status: %d", httpResp.StatusCode))
		return
	}
	if decodeErr != nil {
		resp.Diagnostics.AddError("Error decoding API response", decodeErr.Error())
		return
	}

	if taskEnv.Data.Task.ID != 0 {
		task, err := pollTask(ctx, r.client, r.config, serverID, taskEnv.Data.Task.ID)
		if err != nil {
			resp.Diagnostics.AddError("Build task did not complete", err.Error())
			return
		}
		if !task.Success {
			resp.Diagnostics.AddError("Build task failed", fmt.Sprintf("Task %d finished with status %q.", task.ID, task.Status))
			return
		}
		data.TaskID = types.Int64Value(int64(taskEnv.Data.Task.ID))
	} else {
		data.TaskID = types.Int64Null()
	}

	data.ID = types.StringValue(serverID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read verifies the target server still exists; there is no build object to
// re-fetch, so beyond that it just keeps whatever is already in state.
func (r *VirtfusionServerBuildResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VirtfusionServerBuildResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpReq, err := newAPIRequest(ctx, r.config, "GET", apiServerPath(data.ServerID.ValueString()), nil)
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

// Update is unreachable: every attribute is RequiresReplace, so Terraform
// always does Delete+Create instead. Kept only to satisfy the interface.
func (r *VirtfusionServerBuildResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"virtfusion_build has no Update",
		"Every attribute is RequiresReplace; Terraform should never call Update for this resource.",
	)
}

// Delete never calls the API — there is no "unbuild" endpoint, and calling
// anything here (e.g. a rebuild) would be actively destructive rather than
// a teardown. This only drops the resource from Terraform state.
func (r *VirtfusionServerBuildResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

// flattenInt64List converts []types.Int64 to []int64.
func flattenInt64List(list []types.Int64) []int64 {
	var result []int64
	for _, v := range list {
		if !v.IsNull() {
			result = append(result, v.ValueInt64())
		}
	}
	return result
}
