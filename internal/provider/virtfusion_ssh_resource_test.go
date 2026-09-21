package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func newTestSSHResource(t *testing.T, serverURL string) *VirtfusionSSHResource {
	t.Helper()
	base, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	base.Path = "/api"

	return &VirtfusionSSHResource{
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

func sshSchemaFor(t *testing.T, r *VirtfusionSSHResource) resource.SchemaResponse {
	t.Helper()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	return *schemaResp
}

// TestVirtfusionSSHResource_Create_RequestShape verifies the camelCase
// body, the /account/sshKeys path (no user_id anywhere), and that the new
// key is located by matching public_key in the list-shaped create response.
func TestVirtfusionSSHResource_Create_RequestShape(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		_ = json.NewDecoder(req.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"current_page": 1,
			"data": [
				{"id": 1851, "name": "old key", "publicKey": "ssh-ed25519 OLD", "type": "OpenSSH", "enabled": true, "created": "2024-01-01T00:00:00Z"},
				{"id": 1852, "name": "new key", "publicKey": "ssh-ed25519 NEW", "type": "OpenSSH", "enabled": true, "created": "2024-06-01T00:00:00Z"}
			],
			"next_page_url": null,
			"total": 2
		}`))
	}))
	defer srv.Close()

	r := newTestSSHResource(t, srv.URL)
	ctx := context.Background()

	s := sshSchemaFor(t, r)
	plan := tfsdk.Plan{Schema: s.Schema}
	model := VirtfusionSSHResourceModel{
		Name:      types.StringValue("new key"),
		PublicKey: types.StringValue("ssh-ed25519 NEW"),
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
	if gotPath != "/api/account/sshKeys" {
		t.Errorf("path = %q, want %q", gotPath, "/api/account/sshKeys")
	}
	if gotBody["publicKey"] != "ssh-ed25519 NEW" {
		t.Errorf("body publicKey = %v", gotBody["publicKey"])
	}
	if _, present := gotBody["public_key"]; present {
		t.Error("body should use camelCase publicKey, not public_key")
	}
	if _, present := gotBody["user_id"]; present {
		t.Error("body should not contain user_id — the real API has no such field")
	}

	var got VirtfusionSSHResourceModel
	if diags := createResp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("failed to read back state: %v", diags)
	}
	if got.ID.ValueInt64() != 1852 {
		t.Errorf("state ID = %d, want 1852 (matched by public_key, not first list entry)", got.ID.ValueInt64())
	}
}

// TestVirtfusionSSHResource_Create_EmptyResponseBody_FallsBackToList
// reproduces a real deployment's actual behavior found via live testing:
// POST /account/sshKeys returns 200 with a completely empty body (not the
// documented list envelope), even though the key is created successfully
// server-side. Create must fall back to a fresh list+scan by public_key
// instead of treating the empty/unparsable body as an error.
func TestVirtfusionSSHResource_Create_EmptyResponseBody_FallsBackToList(t *testing.T) {
	postHits, listHits := 0, 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPost:
			postHits++
			w.WriteHeader(http.StatusOK) // empty body, as observed live
		case http.MethodGet:
			listHits++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data": [{"id": 7, "name": "fallback key", "publicKey": "ssh-ed25519 FALLBACK", "type": "OpenSSH", "enabled": true, "created": "2024-01-01T00:00:00Z"}], "next_page_url": null, "total": 1}`))
		}
	}))
	defer srv.Close()

	r := newTestSSHResource(t, srv.URL)
	ctx := context.Background()

	s := sshSchemaFor(t, r)
	plan := tfsdk.Plan{Schema: s.Schema}
	model := VirtfusionSSHResourceModel{
		Name:      types.StringValue("fallback key"),
		PublicKey: types.StringValue("ssh-ed25519 FALLBACK"),
	}
	if diags := plan.Set(ctx, &model); diags.HasError() {
		t.Fatalf("failed to build test plan: %v", diags)
	}

	createResp := &resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, createResp)

	if createResp.Diagnostics.HasError() {
		t.Fatalf("Create returned diagnostics: %v", createResp.Diagnostics)
	}
	if postHits != 1 {
		t.Errorf("POST hits = %d, want 1", postHits)
	}
	if listHits != 1 {
		t.Errorf("fallback GET list hits = %d, want 1", listHits)
	}

	var got VirtfusionSSHResourceModel
	if diags := createResp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("failed to read back state: %v", diags)
	}
	if got.ID.ValueInt64() != 7 {
		t.Errorf("state ID = %d, want 7", got.ID.ValueInt64())
	}
}

// TestVirtfusionSSHResource_Read_FollowsPagination verifies Read paginates
// through next_page_url to find a key on a later page.
func TestVirtfusionSSHResource_Read_FollowsPagination(t *testing.T) {
	var page1Hit, page2Hit bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.URL.Query().Get("page") == "2" {
			page2Hit = true
			_, _ = w.Write([]byte(`{"data": [{"id": 42, "name": "target", "publicKey": "ssh-ed25519 TARGET", "type": "OpenSSH", "enabled": true, "created": "2024-01-01T00:00:00Z"}], "next_page_url": null, "total": 2}`))
			return
		}
		page1Hit = true
		nextURL := "http://" + req.Host + "/api/account/sshKeys?page=2"
		_, _ = w.Write([]byte(fmt.Sprintf(`{"data": [{"id": 1, "name": "other", "publicKey": "ssh-ed25519 OTHER", "type": "OpenSSH", "enabled": true, "created": "2024-01-01T00:00:00Z"}], "next_page_url": %q, "total": 2}`, nextURL)))
	}))
	defer srv.Close()

	r := newTestSSHResource(t, srv.URL)
	ctx := context.Background()

	s := sshSchemaFor(t, r)
	state := tfsdk.State{Schema: s.Schema}
	if diags := state.Set(ctx, &VirtfusionSSHResourceModel{ID: types.Int64Value(42)}); diags.HasError() {
		t.Fatalf("failed to build test state: %v", diags)
	}

	readResp := &resource.ReadResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Read(ctx, resource.ReadRequest{State: state}, readResp)

	if readResp.Diagnostics.HasError() {
		t.Fatalf("Read returned diagnostics: %v", readResp.Diagnostics)
	}
	if !page1Hit || !page2Hit {
		t.Errorf("expected both pages to be hit, page1=%v page2=%v", page1Hit, page2Hit)
	}

	var got VirtfusionSSHResourceModel
	if diags := readResp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("failed to read back state: %v", diags)
	}
	if got.Name.ValueString() != "target" {
		t.Errorf("Name = %q, want %q", got.Name.ValueString(), "target")
	}
}

// TestVirtfusionSSHResource_Delete_RequestShape verifies the correct
// /account/sshKeys/{id} path.
func TestVirtfusionSSHResource_Delete_RequestShape(t *testing.T) {
	var gotPath, gotMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	r := newTestSSHResource(t, srv.URL)
	ctx := context.Background()

	s := sshSchemaFor(t, r)
	state := tfsdk.State{Schema: s.Schema}
	if diags := state.Set(ctx, &VirtfusionSSHResourceModel{ID: types.Int64Value(1851)}); diags.HasError() {
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
	if gotPath != "/api/account/sshKeys/1851" {
		t.Errorf("path = %q, want %q", gotPath, "/api/account/sshKeys/1851")
	}
}
