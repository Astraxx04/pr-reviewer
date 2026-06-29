package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate and inspect the current identity",
	}
	cmd.AddCommand(newLoginCmd(), newWhoamiCmd(), newLogoutCmd())
	return cmd
}

func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Sign in via your browser (GitHub OAuth)",
		Long: "Opens your browser to sign in with GitHub and stores the resulting token in\n" +
			"the config file. This is the only way to authenticate the CLI.\n\n" +
			"Use --server to point at a non-default server (it is saved on success).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return webLogin(cmd.Context())
		},
	}
}

// webLogin runs the loopback OAuth flow: it starts a local listener, opens the
// browser at the server's GitHub login (telling it where to send the token), and
// captures the token the server redirects back. No copy-paste required.
func webLogin(ctx context.Context) error {
	cfg := resolveConfig()
	server := cfg.Server
	if server == "" {
		return fmt.Errorf("no server configured; pass --server")
	}

	// Loopback listener on an OS-assigned port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("starting local listener: %w", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	tokenCh := make(chan string, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		tok := r.URL.Query().Get("token")
		w.Header().Set("Content-Type", "text/html")
		if tok == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, loginResultHTML("Login failed", "No token was returned. You can close this tab and try again."))
			return
		}
		_, _ = fmt.Fprint(w, loginResultHTML("You're signed in", "Authentication complete — return to your terminal. You can close this tab."))
		tokenCh <- tok
	})}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	loginURL := fmt.Sprintf("%s/auth/github?cli_redirect=%s", server, url.QueryEscape(redirectURI))
	fmt.Printf("To sign in with GitHub, open this URL in your browser:\n\n    %s\n\n", loginURL)
	fmt.Print("Trying to open it automatically... ")
	if err := openBrowser(loginURL); err != nil {
		fmt.Println("couldn't — please open the URL above manually.")
	} else {
		fmt.Println("opened.")
	}
	fmt.Println("\nWaiting for authentication to complete (Ctrl-C to cancel)...")

	var token string
	select {
	case token = <-tokenCh:
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(3 * time.Minute):
		return fmt.Errorf("timed out waiting for browser authentication")
	}

	// Persist server + captured token, then verify.
	cfg.Token = token
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	client := newClient(cfg, &http.Client{Timeout: timeoutFlag})
	data, err := client.get(ctx, "/api/auth/me", nil)
	if err != nil {
		return fmt.Errorf("token verification failed: %w", err)
	}
	var me meResponse
	if err := json.Unmarshal(data, &me); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}
	fmt.Printf("Logged in to %s as %s (%s)\n", client.server, me.Login, me.Role)
	return nil
}

// openBrowser opens url in the user's default browser, cross-platform.
func openBrowser(target string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", target).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
	default:
		return exec.Command("xdg-open", target).Start()
	}
}

// loginResultHTML renders the minimal page shown in the browser tab after the
// OAuth round-trip completes.
func loginResultHTML(title, msg string) string {
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><title>%s</title>`+
		`<style>body{font-family:system-ui,sans-serif;display:flex;min-height:100vh;margin:0;`+
		`align-items:center;justify-content:center;background:#0d1117;color:#e6edf3}`+
		`.card{text-align:center;padding:2rem 3rem}.card h1{font-size:1.4rem;margin:0 0 .5rem}`+
		`.card p{color:#9da7b3;margin:0}</style></head>`+
		`<body><div class="card"><h1>%s</h1><p>%s</p></div></body></html>`, title, title, msg)
}

type meResponse struct {
	ID        uint   `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	Role      string `json:"role"`
	Status    string `json:"status"`
}

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the currently authenticated user",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := apiClient.get(cmd.Context(), "/api/auth/me", nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			var me meResponse
			if err := json.Unmarshal(data, &me); err != nil {
				return err
			}
			t := newTable("FIELD", "VALUE")
			t.row("login", me.Login)
			t.row("role", me.Role)
			t.row("status", me.Status)
			t.row("email", me.Email)
			t.flush()
			return nil
		},
	}
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke the current session and clear the stored token",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := apiClient.post(cmd.Context(), "/api/auth/logout", nil)
			if err != nil {
				return err
			}
			// Drop the token from the config file so it is not reused.
			cfg := loadConfig()
			cfg.Token = ""
			if err := saveConfig(cfg); err != nil {
				return fmt.Errorf("clearing token: %w", err)
			}
			fmt.Println("Logged out.")
			return nil
		},
	}
}
