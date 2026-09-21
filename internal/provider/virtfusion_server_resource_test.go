package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// fakeServerFixtureJSON is a synthesized (not real) server object matching
// the field shape observed against the live API in CODE-27.
const fakeServerFixtureJSON = `{
  "data": {
    "id": "11111111-2222-3333-4444-555555555555",
    "name": "fixture-server",
    "hostname": "fixture.example.test",
    "suspended": false,
    "protected": true,
    "migrating": false,
    "deleting": false,
    "backupCreating": false,
    "rescue": false,
    "vncEnabled": true,
    "isoMounted": false,
    "uefi": true,
    "bootOrder": ["hd", "cdrom"],
    "memory": "10240 MB",
    "cpu": "2 Core",
    "storage": [
      {"capacity": "80 GB", "enabled": true, "primary": true, "created": "2024-01-01T00:00:00Z"}
    ],
    "network": {
      "primary": {
        "mac": "aa:bb:cc:dd:ee:ff",
        "limit": "1",
        "ipv4": [{"address": "203.0.113.10", "gateway": "203.0.113.1", "netmask": "255.255.255.0"}],
        "ipv6": [{"subnet": "2001:db8::/64", "gateway": "2001:db8::1", "addresses": ["2001:db8::10"]}]
      },
      "secondary": []
    },
    "currentMonthlyPeriod": {"start": "2024-01-01T00:00:00Z", "end": "2024-02-01T00:00:00Z"},
    "created": "2023-06-15T00:00:00Z",
    "state": null
  }
}`

func TestApiServerToModel_MapsFixtureFields(t *testing.T) {
	var envelope APIServerDetailEnvelope
	if err := json.Unmarshal([]byte(fakeServerFixtureJSON), &envelope); err != nil {
		t.Fatalf("failed to decode fixture: %v", err)
	}

	model := apiServerToModel(envelope.Data)

	if got := model.ID.ValueString(); got != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("ID = %q", got)
	}
	if got := model.Name.ValueString(); got != "fixture-server" {
		t.Errorf("Name = %q", got)
	}
	if !model.Protected.ValueBool() {
		t.Errorf("Protected = false, want true")
	}
	if model.Suspended.ValueBool() {
		t.Errorf("Suspended = true, want false")
	}
	if len(model.BootOrder) != 2 || model.BootOrder[0] != "hd" || model.BootOrder[1] != "cdrom" {
		t.Errorf("BootOrder = %v", model.BootOrder)
	}
	if model.Memory != "10240 MB" {
		t.Errorf("Memory = %q", model.Memory)
	}
	if model.MemoryMB == nil || *model.MemoryMB != 10240 {
		t.Errorf("MemoryMB = %v, want 10240", model.MemoryMB)
	}
	if model.CPU != "2 Core" {
		t.Errorf("CPU = %q", model.CPU)
	}
	if model.CPUCores == nil || *model.CPUCores != 2 {
		t.Errorf("CPUCores = %v, want 2", model.CPUCores)
	}
	if len(model.Storage) != 1 {
		t.Fatalf("Storage len = %d, want 1", len(model.Storage))
	}
	if model.Storage[0].CapacityGB == nil || *model.Storage[0].CapacityGB != 80 {
		t.Errorf("Storage[0].CapacityGB = %v, want 80", model.Storage[0].CapacityGB)
	}
	if model.Network.Primary.MAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("Network.Primary.MAC = %q", model.Network.Primary.MAC)
	}
	if len(model.Network.Primary.IPv4) != 1 || model.Network.Primary.IPv4[0].Address != "203.0.113.10" {
		t.Errorf("Network.Primary.IPv4 = %v", model.Network.Primary.IPv4)
	}
	if len(model.Network.Secondary) != 0 {
		t.Errorf("Network.Secondary len = %d, want 0", len(model.Network.Secondary))
	}
	if model.State != nil {
		t.Errorf("State = %v, want nil (null passthrough)", *model.State)
	}
	if model.CurrentMonthlyPeriod.Start != "2024-01-01T00:00:00Z" {
		t.Errorf("CurrentMonthlyPeriod.Start = %q", model.CurrentMonthlyPeriod.Start)
	}
}

// parseLeadingInt is exercised directly to confirm a bad/unexpected format
// degrades to nil rather than erroring.
func TestParseLeadingInt_UnparseableReturnsNil(t *testing.T) {
	if got := parseLeadingInt("unlimited"); got != nil {
		t.Errorf("parseLeadingInt(unlimited) = %v, want nil", *got)
	}
	if got := parseLeadingInt(""); got != nil {
		t.Errorf("parseLeadingInt(\"\") = %v, want nil", *got)
	}
}

func newTestServerResource(t *testing.T, serverURL string) *VirtfusionServerResource {
	t.Helper()
	base, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	base.Path = "/api"

	return &VirtfusionServerResource{
		client: &http.Client{Transport: &CustomTransport{
			Transport: http.DefaultTransport,
			Token:     "test-token",
		}},
		config: &ProviderConfig{
			Client:   nil,
			Endpoint: base.Host,
			BaseURL:  base,
			ApiToken: "test-token",
		},
	}
}

// stateWithID reproduces the exact shape Terraform core hands Read() right
// after `terraform import`: only "id" is known, every other attribute is
// explicitly null (not merely zero-valued) — this is what previously
// crashed Read when it decoded the full model via State.Get.
func stateWithID(t *testing.T, r *VirtfusionServerResource, id string) tfsdk.State {
	t.Helper()
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	schemaType := schemaResp.Schema.Type().TerraformType(ctx)
	state := tfsdk.State{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(schemaType, nil), // null object: every attribute null
	}
	if diags := state.SetAttribute(ctx, path.Root("id"), id); diags.HasError() {
		t.Fatalf("failed to build test state: %v", diags)
	}
	return state
}

// TestVirtfusionServerResource_Read_RequestShape verifies the fixed URL
// composition (single "/api/server/<id>" path, no doubled host) and that
// exactly one Authorization header is sent, then that the response is
// correctly unwrapped and mapped into state.
func TestVirtfusionServerResource_Read_RequestShape(t *testing.T) {
	const fakeID = "11111111-2222-3333-4444-555555555555"

	var gotPath string
	var authHeaderCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		authHeaderCount = len(req.Header.Values("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fakeServerFixtureJSON))
	}))
	defer srv.Close()

	r := newTestServerResource(t, srv.URL)

	ctx := context.Background()
	readReq := resource.ReadRequest{State: stateWithID(t, r, fakeID)}

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	readResp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Read(ctx, readReq, readResp)

	if readResp.Diagnostics.HasError() {
		t.Fatalf("Read returned diagnostics: %v", readResp.Diagnostics)
	}
	if gotPath != "/api/server/"+fakeID {
		t.Errorf("request path = %q, want %q", gotPath, "/api/server/"+fakeID)
	}
	if authHeaderCount != 1 {
		t.Errorf("Authorization header count = %d, want 1", authHeaderCount)
	}

	var got VirtfusionServerResourceModel
	if diags := readResp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("failed to read back state: %v", diags)
	}
	if got.ID.ValueString() != fakeID {
		t.Errorf("state ID = %q, want %q", got.ID.ValueString(), fakeID)
	}
	if got.Memory != "10240 MB" {
		t.Errorf("state Memory = %q", got.Memory)
	}
	if got.MemoryMB == nil || *got.MemoryMB != 10240 {
		t.Errorf("state MemoryMB = %v", got.MemoryMB)
	}
}

// TestVirtfusionServerResource_MutationsAreGated verifies Create, Update and
// Delete all return the "unverified" diagnostic and never issue a request —
// the core safety property for CODE-27 (no guessed mutating call can ever
// reach the live account).
func TestVirtfusionServerResource_MutationsAreGated(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := newTestServerResource(t, srv.URL)

	ctx := context.Background()

	createResp := &resource.CreateResponse{}
	r.Create(ctx, resource.CreateRequest{}, createResp)
	if !createResp.Diagnostics.HasError() {
		t.Error("Create did not return an error diagnostic")
	} else if createResp.Diagnostics[0].Summary() != unverifiedMutationSummary {
		t.Errorf("Create diagnostic summary = %q", createResp.Diagnostics[0].Summary())
	}

	updateResp := &resource.UpdateResponse{}
	r.Update(ctx, resource.UpdateRequest{}, updateResp)
	if !updateResp.Diagnostics.HasError() {
		t.Error("Update did not return an error diagnostic")
	} else if updateResp.Diagnostics[0].Summary() != unverifiedMutationSummary {
		t.Errorf("Update diagnostic summary = %q", updateResp.Diagnostics[0].Summary())
	}

	deleteResp := &resource.DeleteResponse{}
	r.Delete(ctx, resource.DeleteRequest{}, deleteResp)
	if !deleteResp.Diagnostics.HasError() {
		t.Error("Delete did not return an error diagnostic")
	} else if deleteResp.Diagnostics[0].Summary() != unverifiedMutationSummary {
		t.Errorf("Delete diagnostic summary = %q", deleteResp.Diagnostics[0].Summary())
	}

	if hits != 0 {
		t.Errorf("gated mutation reached the test server %d time(s), want 0", hits)
	}
}
