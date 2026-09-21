package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// stateWithOnlyID reproduces the exact shape Terraform core hands Read()
// right after `terraform import`: only "id" is known, every other
// attribute — including nested SingleNestedAttribute objects like
// "network" — is genuinely null at the object level, not merely
// zero-valued. A plain (non-pointer) Go struct can't represent that; this
// is what caught the real Network/CurrentMonthlyPeriod pointer bug found
// in live testing, which building state via state.Set() from a Go struct
// cannot reproduce (Set() always encodes a struct as "present but
// leaf-null", never as "the whole object is null").
func stateWithOnlyID(t *testing.T, schemaResp resource.SchemaResponse, id string) tfsdk.State {
	t.Helper()
	ctx := context.Background()

	schemaType := schemaResp.Schema.Type().TerraformType(ctx)
	state := tfsdk.State{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(schemaType, nil),
	}
	if diags := state.SetAttribute(ctx, path.Root("id"), id); diags.HasError() {
		t.Fatalf("failed to build test state: %v", diags)
	}
	return state
}

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
        "limit": "2000 GB",
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
			Endpoint: base.Host,
			BaseURL:  base,
			ApiToken: "test-token",
		},
	}
}

func serverSchemaFor(t *testing.T, r *VirtfusionServerResource) resource.SchemaResponse {
	t.Helper()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	return *schemaResp
}

func TestNormalizeBootOrder(t *testing.T) {
	cases := map[string]string{
		"hd,cdrom":  "hdd,cdrom",
		"cdrom,hd":  "cdrom,hdd",
		"hdd,cdrom": "hdd,cdrom",
	}
	for in, want := range cases {
		got := normalizeBootOrder(strings.Split(in, ","))
		if got != want {
			t.Errorf("normalizeBootOrder(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestVirtfusionServerResource_Read_RequestShape verifies the composed URL,
// single Authorization header, and that the response is fully mapped into
// state (the systemic "Read discards the response" bug both independent
// reviews flagged).
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

	s := serverSchemaFor(t, r)
	state := stateWithOnlyID(t, s, fakeID)

	readResp := &resource.ReadResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Read(ctx, resource.ReadRequest{State: state}, readResp)

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
	if got.Memory.ValueString() != "10240 MB" {
		t.Errorf("Memory = %q, want %q", got.Memory.ValueString(), "10240 MB")
	}
	if got.BootOrder.ValueString() != "hdd,cdrom" {
		t.Errorf("BootOrder = %q, want %q", got.BootOrder.ValueString(), "hdd,cdrom")
	}
	if len(got.Storage) != 1 || got.Storage[0].Capacity.ValueString() != "80 GB" {
		t.Errorf("Storage = %+v", got.Storage)
	}
	if got.Network.Primary.MAC.ValueString() != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("Network.Primary.MAC = %q", got.Network.Primary.MAC.ValueString())
	}
	if len(got.Network.Primary.IPv4) != 1 || got.Network.Primary.IPv4[0].Address.ValueString() != "203.0.113.10" {
		t.Errorf("Network.Primary.IPv4 = %+v", got.Network.Primary.IPv4)
	}
	if !got.State.IsNull() {
		t.Errorf("State = %v, want null", got.State)
	}
}

// TestVirtfusionServerResource_Create_RequestShape verifies the discovery
// flow's create call: POST /resourcePack/{id}/{createId}, correct override
// body, and the bare (unwrapped) {"id": ...} response handled correctly,
// followed by a GET to populate the rest of the model.
func TestVirtfusionServerResource_Create_RequestShape(t *testing.T) {
	const fakeID = "22222222-3333-4444-5555-666666666666"

	var createPath, createMethod string
	var createBody map[string]interface{}
	var getPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodPost:
			createPath = req.URL.Path
			createMethod = req.Method
			_ = json.NewDecoder(req.Body).Decode(&createBody)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": "` + fakeID + `"}`))
		case req.Method == http.MethodGet:
			getPath = req.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data": {"id": "` + fakeID + `", "network": {"primary": {}, "secondary": []}, "currentMonthlyPeriod": {}}}`))
		}
	}))
	defer srv.Close()

	r := newTestServerResource(t, srv.URL)
	ctx := context.Background()

	s := serverSchemaFor(t, r)
	plan := tfsdk.Plan{Schema: s.Schema}
	model := VirtfusionServerResourceModel{
		ResourcePackID:   types.Int64Value(5),
		CreateID:         types.StringValue("abc123"),
		OverrideMemoryMB: types.Int64Value(2048),
	}
	if diags := plan.Set(ctx, &model); diags.HasError() {
		t.Fatalf("failed to build test plan: %v", diags)
	}

	createResp := &resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, createResp)

	if createResp.Diagnostics.HasError() {
		t.Fatalf("Create returned diagnostics: %v", createResp.Diagnostics)
	}
	if createMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", createMethod)
	}
	if createPath != "/api/resourcePack/5/abc123" {
		t.Errorf("create path = %q, want %q", createPath, "/api/resourcePack/5/abc123")
	}
	if createBody["memory"] != float64(2048) {
		t.Errorf("create body memory = %v, want 2048", createBody["memory"])
	}
	if _, present := createBody["storage"]; present {
		t.Errorf("create body should omit unset storage override, got %v", createBody["storage"])
	}
	if getPath != "/api/server/"+fakeID {
		t.Errorf("follow-up GET path = %q, want %q", getPath, "/api/server/"+fakeID)
	}

	var got VirtfusionServerResourceModel
	if diags := createResp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("failed to read back state: %v", diags)
	}
	if got.ID.ValueString() != fakeID {
		t.Errorf("state ID = %q, want %q", got.ID.ValueString(), fakeID)
	}
}

// TestVirtfusionServerResource_Create_RequiresResourcePackAndCreateID
// verifies Create fails fast, without any HTTP call, when the (Optional so
// import stays safe) discovery fields aren't set.
func TestVirtfusionServerResource_Create_RequiresResourcePackAndCreateID(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hits++
	}))
	defer srv.Close()

	r := newTestServerResource(t, srv.URL)
	ctx := context.Background()

	s := serverSchemaFor(t, r)
	plan := tfsdk.Plan{Schema: s.Schema}
	if diags := plan.Set(ctx, &VirtfusionServerResourceModel{}); diags.HasError() {
		t.Fatalf("failed to build test plan: %v", diags)
	}

	createResp := &resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, createResp)

	if !createResp.Diagnostics.HasError() {
		t.Error("Create with unset resource_pack_id/create_id did not return an error")
	}
	if hits != 0 {
		t.Errorf("Create reached the test server %d time(s), want 0", hits)
	}
}

// TestVirtfusionServerResource_Delete_RequestShape verifies Delete targets
// DELETE /resourcePack/{id}, not /server/{id}.
func TestVirtfusionServerResource_Delete_RequestShape(t *testing.T) {
	const fakeID = "33333333-4444-5555-6666-777777777777"

	var gotPath, gotMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	r := newTestServerResource(t, srv.URL)
	ctx := context.Background()

	s := serverSchemaFor(t, r)
	state := tfsdk.State{Schema: s.Schema}
	if diags := state.Set(ctx, &VirtfusionServerResourceModel{ID: types.StringValue(fakeID)}); diags.HasError() {
		t.Fatalf("failed to build test state: %v", diags)
	}

	deleteResp := &resource.DeleteResponse{}
	r.Delete(ctx, resource.DeleteRequest{State: state}, deleteResp)

	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("Delete returned diagnostics: %v", deleteResp.Diagnostics)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/api/resourcePack/"+fakeID {
		t.Errorf("request path = %q, want %q", gotPath, "/api/resourcePack/"+fakeID)
	}
}

// TestVirtfusionServerResource_Update_Name verifies a name change goes to
// PUT /server/{id}/name, and nothing else is called when only name changed.
func TestVirtfusionServerResource_Update_Name(t *testing.T) {
	const fakeID = "44444444-5555-6666-7777-888888888888"

	var putPath, putMethod string
	var putBody map[string]interface{}
	getCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPut:
			putPath = req.URL.Path
			putMethod = req.Method
			_ = json.NewDecoder(req.Body).Decode(&putBody)
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			getCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data": {"id": "` + fakeID + `", "name": "new-name", "network": {"primary": {}, "secondary": []}, "currentMonthlyPeriod": {}}}`))
		}
	}))
	defer srv.Close()

	r := newTestServerResource(t, srv.URL)
	ctx := context.Background()

	s := serverSchemaFor(t, r)

	stateModel := VirtfusionServerResourceModel{ID: types.StringValue(fakeID), Name: types.StringValue("old-name")}
	state := tfsdk.State{Schema: s.Schema}
	if diags := state.Set(ctx, &stateModel); diags.HasError() {
		t.Fatalf("failed to build test state: %v", diags)
	}

	planModel := stateModel
	planModel.Name = types.StringValue("new-name")
	plan := tfsdk.Plan{Schema: s.Schema}
	if diags := plan.Set(ctx, &planModel); diags.HasError() {
		t.Fatalf("failed to build test plan: %v", diags)
	}

	updateResp := &resource.UpdateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, updateResp)

	if updateResp.Diagnostics.HasError() {
		t.Fatalf("Update returned diagnostics: %v", updateResp.Diagnostics)
	}
	if putMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", putMethod)
	}
	if putPath != "/api/server/"+fakeID+"/name" {
		t.Errorf("PUT path = %q, want %q", putPath, "/api/server/"+fakeID+"/name")
	}
	if putBody["name"] != "new-name" {
		t.Errorf("PUT body name = %v, want new-name", putBody["name"])
	}
	if getCalls != 1 {
		t.Errorf("GET calls = %d, want 1 (one refresh after the update)", getCalls)
	}
}
