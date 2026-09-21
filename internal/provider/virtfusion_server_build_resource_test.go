package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func newTestBuildResource(t *testing.T, serverURL string) *VirtfusionServerBuildResource {
	t.Helper()
	base, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	base.Path = "/api"

	return &VirtfusionServerBuildResource{
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

func buildSchemaFor(t *testing.T, r *VirtfusionServerBuildResource) resource.SchemaResponse {
	t.Helper()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	return *schemaResp
}

// TestVirtfusionServerBuildResource_Create_RequestShape verifies the build
// call targets POST /server/{id}/build with a camelCase body, polls the
// returned task until completion, and sets id = server_id (build has no
// identity of its own).
func TestVirtfusionServerBuildResource_Create_RequestShape(t *testing.T) {
	const serverID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	var buildPath string
	var buildBody map[string]interface{}
	taskPolls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodPost:
			buildPath = req.URL.Path
			_ = json.NewDecoder(req.Body).Decode(&buildBody)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"data":{"task":{"id":99,"status":"accepted"}}}`))
		case req.Method == http.MethodGet:
			taskPolls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":99,"action":"build","completed":true,"status":"complete","success":true}`))
		}
	}))
	defer srv.Close()

	r := newTestBuildResource(t, srv.URL)
	ctx := context.Background()

	s := buildSchemaFor(t, r)
	plan := tfsdk.Plan{Schema: s.Schema}
	model := VirtfusionServerBuildResourceModel{
		ServerID:   types.StringValue(serverID),
		Method:     types.StringValue("template"),
		TemplateID: types.Int64Value(21),
		SSHKeys:    []types.Int64{types.Int64Value(1851)},
	}
	if diags := plan.Set(ctx, &model); diags.HasError() {
		t.Fatalf("failed to build test plan: %v", diags)
	}

	createResp := &resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, createResp)

	if createResp.Diagnostics.HasError() {
		t.Fatalf("Create returned diagnostics: %v", createResp.Diagnostics)
	}
	if buildPath != "/api/server/"+serverID+"/build" {
		t.Errorf("build path = %q, want %q", buildPath, "/api/server/"+serverID+"/build")
	}
	if buildBody["templateId"] != float64(21) {
		t.Errorf("build body templateId = %v, want 21", buildBody["templateId"])
	}
	if _, present := buildBody["osid"]; present {
		t.Error("build body should not contain the fictional osid field")
	}
	sshKeys, ok := buildBody["sshKeys"].([]interface{})
	if !ok || len(sshKeys) != 1 || sshKeys[0] != float64(1851) {
		t.Errorf("build body sshKeys = %v, want [1851]", buildBody["sshKeys"])
	}
	if taskPolls != 1 {
		t.Errorf("task poll count = %d, want 1", taskPolls)
	}

	var got VirtfusionServerBuildResourceModel
	if diags := createResp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("failed to read back state: %v", diags)
	}
	if got.ID.ValueString() != serverID {
		t.Errorf("state ID = %q, want %q (build has no identity of its own)", got.ID.ValueString(), serverID)
	}
	if got.TaskID.ValueInt64() != 99 {
		t.Errorf("state TaskID = %d, want 99", got.TaskID.ValueInt64())
	}
}

// TestVirtfusionServerBuildResource_Create_RequiresTemplateID verifies
// Create fails fast without any HTTP call when method=template but
// template_id is unset.
func TestVirtfusionServerBuildResource_Create_RequiresTemplateID(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hits++
	}))
	defer srv.Close()

	r := newTestBuildResource(t, srv.URL)
	ctx := context.Background()

	s := buildSchemaFor(t, r)
	plan := tfsdk.Plan{Schema: s.Schema}
	model := VirtfusionServerBuildResourceModel{
		ServerID: types.StringValue("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"),
		Method:   types.StringValue("template"),
	}
	if diags := plan.Set(ctx, &model); diags.HasError() {
		t.Fatalf("failed to build test plan: %v", diags)
	}

	createResp := &resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, createResp)

	if !createResp.Diagnostics.HasError() {
		t.Error("Create with method=template and unset template_id did not return an error")
	}
	if hits != 0 {
		t.Errorf("Create reached the test server %d time(s), want 0", hits)
	}
}

// TestVirtfusionServerBuildResource_Delete_NeverCallsAPI verifies Delete
// only drops Terraform state — there is no "unbuild" endpoint to call.
func TestVirtfusionServerBuildResource_Delete_NeverCallsAPI(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hits++
	}))
	defer srv.Close()

	r := newTestBuildResource(t, srv.URL)
	ctx := context.Background()

	deleteResp := &resource.DeleteResponse{}
	r.Delete(ctx, resource.DeleteRequest{}, deleteResp)

	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("Delete returned diagnostics: %v", deleteResp.Diagnostics)
	}
	if hits != 0 {
		t.Errorf("Delete reached the test server %d time(s), want 0", hits)
	}
}
