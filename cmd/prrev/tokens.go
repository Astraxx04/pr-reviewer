package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newTokensCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "Manage API tokens for programmatic access",
	}
	cmd.AddCommand(newTokensListCmd(), newTokensCreateCmd(), newTokensRevokeCmd())
	return cmd
}

func newTokensListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List your API tokens",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := apiClient.get(cmd.Context(), "/api/tokens", nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			var tokens []struct {
				ID         uint       `json:"ID"`
				Name       string     `json:"Name"`
				Scope      string     `json:"Scope"`
				Prefix     string     `json:"Prefix"`
				LastUsedAt *time.Time `json:"LastUsedAt"`
				ExpiresAt  *time.Time `json:"ExpiresAt"`
			}
			if err := json.Unmarshal(data, &tokens); err != nil {
				return err
			}
			t := newTable("ID", "NAME", "SCOPE", "PREFIX", "LAST USED", "EXPIRES")
			for _, tok := range tokens {
				t.row(fmt.Sprint(tok.ID), tok.Name, tok.Scope, tok.Prefix,
					fmtTime(tok.LastUsedAt), fmtTime(tok.ExpiresAt))
			}
			t.flush()
			return nil
		},
	}
}

func newTokensCreateCmd() *cobra.Command {
	var name, scope string
	var expiresIn int
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an API token (the raw token is shown only once)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if scope != "read" && scope != "readwrite" {
				return fmt.Errorf("--scope must be 'read' or 'readwrite'")
			}
			data, err := apiClient.post(cmd.Context(), "/api/tokens", map[string]any{
				"name":       name,
				"scope":      scope,
				"expires_in": expiresIn,
			})
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			var res struct {
				Token  string `json:"token"`
				Prefix string `json:"prefix"`
				Scope  string `json:"scope"`
			}
			if err := json.Unmarshal(data, &res); err != nil {
				return err
			}
			fmt.Printf("API token created (scope: %s).\n", res.Scope)
			fmt.Printf("\n    %s\n\n", res.Token)
			fmt.Println("This is the only time the token is shown — store it securely.")
			fmt.Println("Use it as a Bearer token in the Authorization header for API access.")
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&name, "name", "", "human-readable token name (required)")
	f.StringVar(&scope, "scope", "read", "token scope: read | readwrite")
	f.IntVar(&expiresIn, "expires-in", 0, "expiry in days (0 = never)")
	return cmd
}

func newTokensRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke an API token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			data, err := apiClient.delete(cmd.Context(), fmt.Sprintf("/api/tokens/%d", id))
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			fmt.Printf("Token %d revoked.\n", id)
			return nil
		},
	}
}
