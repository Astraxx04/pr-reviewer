package main

import (
	"encoding/json"
	"fmt"

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
		Short: "Save server URL and token to the config file, then verify them",
		Long: "Persists --server/--token to the config file and confirms they work by\n" +
			"calling /api/auth/me. Token may be a JWT or an API token (prt_…).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Persist the explicitly-provided flags into the config file.
			cfg := loadConfig()
			if serverFlag != "" {
				cfg.Server = serverFlag
			}
			if tokenFlag != "" {
				cfg.Token = tokenFlag
			}
			if err := saveConfig(cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			// Verify using the resolved client (config + env + flags).
			data, err := apiClient.get(cmd.Context(), "/api/auth/me", nil)
			if err != nil {
				return fmt.Errorf("token verification failed: %w", err)
			}
			var me meResponse
			if err := json.Unmarshal(data, &me); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}
			fmt.Printf("Logged in to %s as %s (%s)\n", apiClient.server, me.Login, me.Role)
			return nil
		},
	}
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
