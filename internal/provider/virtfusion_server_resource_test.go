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
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

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

func schemaFor(t *testing.T, r *VirtfusionServerResource) resource.SchemaResponse {
	t.Helper()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	return *schemaResp
}

// stateWithID reproduces the exact shape Terraform core hands Read() right
// after `terraform import`: only "id" is known, every other attribute is
// explicitly null (not merely zero-valued).
func stateWithID(t *testing.T, r *VirtfusionServerResource, id string) tfsdk.State {
	t.Helper()
	ctx := context.Background()

	s := schemaFor(t, r)
	schemaType := s.Schema.Type().TerraformType(ctx)
	state := tfsdk.State{
		Schema: s.Schema,
		Raw:    tftypes.NewValue(schemaType, nil), // null object: every attribute null
	}
	if diags := state.SetAttribute(ctx, path.Root("id"), id); diags.HasError() {
		t.Fatalf("failed to build test state: %v", diags)
	}
	return state
}

// TestVirtfusionServerResource_Read_RequestShape verifies the fixed URL
// composition (single "/api/server/<id>" path, no doubled host) and that
// exactly one Authorization header is sent.
func TestVirtfusionServerResource_Read_RequestShape(t *testing.T) {
	const fakeID = "11111111-2222-3333-4444-555555555555"

	var gotPath string
	var authHeaderCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		authHeaderCount = len(req.Header.Values("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"id": "` + fakeID + `"}}`))
	}))
	defer srv.Close()

	r := newTestServerResource(t, srv.URL)

	ctx := context.Background()
	readReq := resource.ReadRequest{State: stateWithID(t, r, fakeID)}

	s := schemaFor(t, r)
	readResp := &resource.ReadResponse{State: tfsdk.State{Schema: s.Schema}}

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
}

// TestVirtfusionServerResource_Create_RequestShape verifies Create composes
// the correct URL, sends a single Authorization header, and captures the
// returned (string/UUID) id.
func TestVirtfusionServerResource_Create_RequestShape(t *testing.T) {
	const fakeID = "22222222-3333-4444-5555-666666666666"

	var gotPath, gotMethod string
	var authHeaderCount int
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		authHeaderCount = len(req.Header.Values("Authorization"))
		_ = json.NewDecoder(req.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": "` + fakeID + `"}`))
	}))
	defer srv.Close()

	r := newTestServerResource(t, srv.URL)
	ctx := context.Background()

	s := schemaFor(t, r)
	plan := tfsdk.Plan{Schema: s.Schema}
	model := VirtfusionServerResourceModel{
		UserID: types.Int64Value(1),
	}
	if diags := plan.Set(ctx, &model); diags.HasError() {
		t.Fatalf("failed to build test plan: %v", diags)
	}

	createResp := &resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, createResp)

	if createResp.Diagnostics.HasError() {
		t.Fatalf("Create returned diagnostics: %v", createResp.Diagnostics)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v1/servers" {
		t.Errorf("request path = %q, want %q", gotPath, "/api/v1/servers")
	}
	if authHeaderCount != 1 {
		t.Errorf("Authorization header count = %d, want 1", authHeaderCount)
	}
	if gotBody["user_id"] != float64(1) {
		t.Errorf("request body user_id = %v, want 1", gotBody["user_id"])
	}

	var got VirtfusionServerResourceModel
	if diags := createResp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("failed to read back state: %v", diags)
	}
	if got.ID.ValueString() != fakeID {
		t.Errorf("state ID = %q, want %q", got.ID.ValueString(), fakeID)
	}
}

// TestVirtfusionServerResource_Delete_RequestShape verifies Delete composes
// the correct URL for a UUID id and sends a single Authorization header.
func TestVirtfusionServerResource_Delete_RequestShape(t *testing.T) {
	const fakeID = "33333333-4444-5555-6666-777777777777"

	var gotPath, gotMethod string
	var authHeaderCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		authHeaderCount = len(req.Header.Values("Authorization"))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	r := newTestServerResource(t, srv.URL)
	ctx := context.Background()

	s := schemaFor(t, r)
	state := tfsdk.State{Schema: s.Schema}
	model := VirtfusionServerResourceModel{ID: types.StringValue(fakeID)}
	if diags := state.Set(ctx, &model); diags.HasError() {
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
	if gotPath != "/api/v1/servers/"+fakeID {
		t.Errorf("request path = %q, want %q", gotPath, "/api/v1/servers/"+fakeID)
	}
	if authHeaderCount != 1 {
		t.Errorf("Authorization header count = %d, want 1", authHeaderCount)
	}
}
