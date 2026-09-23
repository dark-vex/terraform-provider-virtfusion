// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"
)

// newAPIRequest builds an *http.Request against the provider's configured
// API base URL. relPath is always a path relative to that base (e.g.
// "/server" or "/server/"+id) — callers must never concatenate the
// endpoint/scheme themselves. Authentication is applied centrally by
// CustomTransport, not here.
func newAPIRequest(ctx context.Context, cfg *ProviderConfig, method, relPath string, body []byte) (*http.Request, error) {
	u := *cfg.BaseURL
	u.Path = path.Join(cfg.BaseURL.Path, relPath)

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// apiServerPath returns the relative path for a single server resource,
// safely escaping the (UUID) id.
func apiServerPath(id string) string {
	return path.Join("/server", url.PathEscape(id))
}

// newAPIRequestAbsolute builds a request against an already-fully-qualified
// URL, such as a pagination next_page_url the API handed back in a prior
// response. Unlike newAPIRequest it does not compose against BaseURL, but
// it still goes through the same authenticated client.
func newAPIRequestAbsolute(ctx context.Context, method, absoluteURL string, body []byte) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, absoluteURL, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// pollTask polls GET /server/{serverID}/task/{taskID} until the task
// reports completed=true, or ctx is done, or a bounded number of attempts
// is exhausted. Several mutating endpoints (build, bootOrder, ...) only
// trigger an async job and return "accepted" immediately — this is how the
// real outcome of that job is observed.
func pollTask(ctx context.Context, client *http.Client, cfg *ProviderConfig, serverID string, taskID int) (*APITask, error) {
	relPath := path.Join(apiServerPath(serverID), "task", fmt.Sprintf("%d", taskID))

	const maxAttempts = 60
	const interval = 5 * time.Second

	for attempt := 0; attempt < maxAttempts; attempt++ {
		req, err := newAPIRequest(ctx, cfg, "GET", relPath, nil)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		// Confirmed live: unlike the mutating endpoints that trigger a task
		// (which wrap it as {"data":{"task": {...}}}, see APITaskEnvelope),
		// GET /server/{serverId}/task/{taskId} wraps it one level shallower
		// as {"data": {...}} — not fully unwrapped as previously assumed.
		// Decoding straight into APITask left every field (including
		// Completed) at its zero value, so this loop never observed
		// completion and always ran out the full 60-attempt budget even
		// when the task had actually finished on the first poll.
		var envelope struct {
			Data APITask `json:"data"`
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&envelope)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status %d while polling task %d", resp.StatusCode, taskID)
		}
		if decodeErr != nil {
			return nil, decodeErr
		}

		if envelope.Data.Completed {
			task := envelope.Data
			return &task, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}

	return nil, fmt.Errorf("task %d did not complete after %d polling attempts", taskID, maxAttempts)
}
