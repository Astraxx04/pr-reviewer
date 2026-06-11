package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client is a thin HTTP client for the PR Reviewer API.
type Client struct {
	server string
	token  string
	http   *http.Client
}

func newClient(cfg config, httpClient *http.Client) *Client {
	return &Client{
		server: strings.TrimRight(cfg.Server, "/"),
		token:  cfg.Token,
		http:   httpClient,
	}
}

// APIError is returned for any non-2xx response. It carries the status code and
// the server's error message (parsed from the {"error":"..."} envelope when present).
type APIError struct {
	Status  int
	Message string
	Body    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("server returned %d: %s", e.Status, e.Message)
	}
	if e.Body != "" {
		return fmt.Sprintf("server returned %d: %s", e.Status, e.Body)
	}
	return fmt.Sprintf("server returned %d", e.Status)
}

// request performs an HTTP request and returns the raw response body. A non-2xx
// status is converted into an *APIError. body may be nil.
func (c *Client) request(ctx context.Context, method, path string, query url.Values, body any) ([]byte, error) {
	if c.token == "" {
		return nil, fmt.Errorf("not authenticated: run `pr-reviewer-cli auth login --token <token>` or set PR_REVIEWER_TOKEN")
	}

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	u := c.server + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// Surface context cancellation/timeout cleanly.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		apiErr := &APIError{Status: resp.StatusCode, Body: strings.TrimSpace(string(data))}
		var env struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &env) == nil && env.Error != "" {
			apiErr.Message = env.Error
		}
		return nil, apiErr
	}
	return data, nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	return c.request(ctx, http.MethodGet, path, query, nil)
}

func (c *Client) post(ctx context.Context, path string, body any) ([]byte, error) {
	return c.request(ctx, http.MethodPost, path, nil, body)
}

func (c *Client) patch(ctx context.Context, path string, body any) ([]byte, error) {
	return c.request(ctx, http.MethodPatch, path, nil, body)
}

func (c *Client) delete(ctx context.Context, path string) ([]byte, error) {
	return c.request(ctx, http.MethodDelete, path, nil, nil)
}
