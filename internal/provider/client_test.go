package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func newTestClientConfig(t *testing.T, serverURL string) (*http.Client, *ProviderConfig) {
	t.Helper()
	base, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	base.Path = "/api"

	client := &http.Client{Transport: &CustomTransport{
		Transport: http.DefaultTransport,
		Token:     "test-token",
	}}
	cfg := &ProviderConfig{
		Endpoint: base.Host,
		BaseURL:  base,
		ApiToken: "test-token",
	}
	return client, cfg
}

// TestPollTask_WrappedResponse verifies pollTask decodes the real, confirmed
// live response shape for GET /server/{serverId}/task/{taskId} —
// {"data": {...}} — rather than the bare object previously assumed. Decoding
// straight into APITask (skipping the "data" envelope) left Completed at its
// zero value forever, so a task that finished on the very first poll would
// still exhaust all 60 attempts and fail with a spurious timeout.
func TestPollTask_WrappedResponse(t *testing.T) {
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		polls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":42,"action":"build","completed":true,"status":"complete","success":true}}`))
	}))
	defer srv.Close()

	client, cfg := newTestClientConfig(t, srv.URL)

	task, err := pollTask(context.Background(), client, cfg, "server-1", 42)
	if err != nil {
		t.Fatalf("pollTask returned error: %v", err)
	}
	if polls != 1 {
		t.Errorf("poll count = %d, want 1 (should return on first completed poll, not exhaust all attempts)", polls)
	}
	if task == nil || !task.Completed {
		t.Fatalf("task = %+v, want a completed task", task)
	}
	if task.ID != 42 {
		t.Errorf("task.ID = %d, want 42", task.ID)
	}
}
