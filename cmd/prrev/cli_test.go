package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParsePRRef(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in                  string
		wantOwner, wantRepo string
		wantNum             int
		wantErr             bool
	}{
		{"demo-org/api-service#42", "demo-org", "api-service", 42, false},
		{"a/b#1", "a", "b", 1, false},
		{"no-hash", "", "", 0, true},
		{"missing-slash#3", "", "", 0, true},
		{"owner/repo#0", "", "", 0, true},
		{"owner/repo#-2", "", "", 0, true},
		{"owner/repo#abc", "", "", 0, true},
		{"/repo#1", "", "", 0, true},
		{"owner/#1", "", "", 0, true},
	}
	for _, c := range cases {
		o, r, n, err := parsePRRef(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parsePRRef(%q): expected error, got nil", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePRRef(%q): unexpected error %v", c.in, err)
			continue
		}
		if o != c.wantOwner || r != c.wantRepo || n != c.wantNum {
			t.Errorf("parsePRRef(%q) = (%q,%q,%d), want (%q,%q,%d)",
				c.in, o, r, n, c.wantOwner, c.wantRepo, c.wantNum)
		}
	}
}

// resetFlags clears the global flag state between tests.
func resetFlags() {
	serverFlag, configFlag = "", ""
}

func TestResolveConfigPrecedence(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgFile, []byte(`{"server":"http://file:1","token":"file-tok"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("file only", func(t *testing.T) {
		resetFlags()
		configFlag = cfgFile
		cfg := resolveConfig()
		if cfg.Server != "http://file:1" || cfg.Token != "file-tok" {
			t.Errorf("got %+v", cfg)
		}
	})

	t.Run("server flag overrides file; token stays from file", func(t *testing.T) {
		resetFlags()
		configFlag = cfgFile
		serverFlag = "http://flag:3"
		defer resetFlags()
		cfg := resolveConfig()
		if cfg.Server != "http://flag:3" {
			t.Errorf("server = %q, want http://flag:3", cfg.Server)
		}
		// The token is only ever sourced from the config file.
		if cfg.Token != "file-tok" {
			t.Errorf("token = %q, want file-tok", cfg.Token)
		}
	})

	t.Run("default server when nothing set", func(t *testing.T) {
		resetFlags()
		configFlag = filepath.Join(dir, "does-not-exist.json")
		cfg := resolveConfig()
		if cfg.Server != defaultServer {
			t.Errorf("server = %q, want default %q", cfg.Server, defaultServer)
		}
	})
}

func TestClientRequestSendsBearerAndParsesJSON(t *testing.T) {
	var gotAuth, gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login":"alice","role":"owner"}`))
	}))
	defer srv.Close()

	c := newClient(config{Server: srv.URL, Token: "prt_secret"}, &http.Client{Timeout: 5 * time.Second})
	data, err := c.get(context.Background(), "/api/auth/me", nil)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if gotAuth != "Bearer prt_secret" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer prt_secret")
	}
	if gotMethod != http.MethodGet || gotPath != "/api/auth/me" {
		t.Errorf("got %s %s", gotMethod, gotPath)
	}
	if string(data) != `{"login":"alice","role":"owner"}` {
		t.Errorf("body = %s", data)
	}
}

func TestClientParsesErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"pull request not found"}`))
	}))
	defer srv.Close()

	c := newClient(config{Server: srv.URL, Token: "x"}, &http.Client{Timeout: 5 * time.Second})
	_, err := c.get(context.Background(), "/api/prs/a/b/9", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", apiErr.Status)
	}
	if apiErr.Message != "pull request not found" {
		t.Errorf("message = %q", apiErr.Message)
	}
}

func TestClientRequiresToken(t *testing.T) {
	c := newClient(config{Server: "http://x"}, &http.Client{})
	if _, err := c.get(context.Background(), "/api/repos", nil); err == nil {
		t.Fatal("expected error when token is empty")
	}
}
