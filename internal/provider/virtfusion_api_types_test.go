package provider

import (
	"encoding/json"
	"testing"
)

// TestAPITask_SuccessToleratesEmptyString reproduces a real deployment's
// live behavior: GET /server/{serverId}/task/{taskId} returns "success" as
// an empty string ("") while a task is still in progress, only becoming a
// real JSON boolean once it completes. A plain `bool` field fails to decode
// entirely on the in-progress shape, which broke every poll before the task
// finished.
func TestAPITask_SuccessToleratesEmptyString(t *testing.T) {
	var inProgress APITask
	if err := json.Unmarshal([]byte(`{"id":1,"completed":false,"status":"in progress","success":""}`), &inProgress); err != nil {
		t.Fatalf("failed to decode in-progress task with empty-string success: %v", err)
	}
	if inProgress.Success {
		t.Error("in-progress task with success:\"\" decoded as true, want false")
	}

	var done APITask
	if err := json.Unmarshal([]byte(`{"id":1,"completed":true,"status":"complete","success":true}`), &done); err != nil {
		t.Fatalf("failed to decode completed task with boolean success: %v", err)
	}
	if !done.Success {
		t.Error("completed task with success:true decoded as false")
	}
}
