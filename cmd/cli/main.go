// pr-reviewer-cli — command-line interface for the PR Reviewer service.
//
// Authentication uses a Bearer token. For CI/automation, mint a long-lived API
// token (prefix "prt_") with `pr-reviewer-cli tokens create` and supply it via
// --token, the PR_REVIEWER_TOKEN env var, or `auth login`.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

// Global flags (persistent across all subcommands).
var (
	serverFlag  string
	tokenFlag   string
	configFlag  string
	jsonOut     bool
	timeoutFlag time.Duration
)

// apiClient is built once in PersistentPreRunE and shared by all commands.
var apiClient *Client

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "pr-reviewer-cli",
		Short:         "CLI for the PR Reviewer service",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg := resolveConfig()
			apiClient = newClient(cfg, &http.Client{Timeout: timeoutFlag})
			return nil
		},
	}

	pf := root.PersistentFlags()
	pf.StringVar(&serverFlag, "server", "", "server URL (overrides config / PR_REVIEWER_SERVER)")
	pf.StringVar(&tokenFlag, "token", "", "bearer token (overrides config / PR_REVIEWER_TOKEN)")
	pf.StringVar(&configFlag, "config", "", "config file path (default ~/.config/pr-reviewer/config.json)")
	pf.BoolVar(&jsonOut, "json", false, "output raw JSON instead of tables")
	pf.DurationVar(&timeoutFlag, "timeout", 30*time.Second, "HTTP request timeout")

	root.AddCommand(
		newAuthCmd(),
		newReposCmd(),
		newPRsCmd(),
		newReviewsCmd(),
		newProvidersCmd(),
		newDashboardCmd(),
		newTokensCmd(),
	)
	return root
}

func main() {
	// Cancel in-flight requests on Ctrl-C / SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
