// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"path"
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
