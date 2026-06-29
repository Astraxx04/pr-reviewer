package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newProvidersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "List, test, and check the health of AI providers",
	}
	cmd.AddCommand(newProvidersListCmd(), newProvidersTestCmd(), newProvidersHealthCmd())
	return cmd
}

func newProvidersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured AI providers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := apiClient.get(cmd.Context(), "/api/providers", nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			var providers []struct {
				ID                 uint   `json:"id"`
				Name               string `json:"name"`
				Type               string `json:"type"`
				DefaultModel       string `json:"default_model"`
				SupportsEmbeddings bool   `json:"supports_embeddings"`
				HasAPIKey          bool   `json:"has_api_key"`
			}
			if err := json.Unmarshal(data, &providers); err != nil {
				return err
			}
			t := newTable("ID", "NAME", "TYPE", "MODEL", "EMBEDDINGS", "KEY")
			for _, p := range providers {
				t.row(fmt.Sprint(p.ID), p.Name, p.Type, orDash(p.DefaultModel),
					yesNo(p.SupportsEmbeddings), yesNo(p.HasAPIKey))
			}
			t.flush()
			return nil
		},
	}
}

func newProvidersTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <id>",
		Short: "Test connectivity to a provider and record its health",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			data, err := apiClient.post(cmd.Context(), fmt.Sprintf("/api/providers/%d/test", id), nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			var res struct {
				OK        bool   `json:"ok"`
				Message   string `json:"message"`
				LatencyMS int64  `json:"latency_ms"`
			}
			if err := json.Unmarshal(data, &res); err != nil {
				return err
			}
			status := "FAILED"
			if res.OK {
				status = "OK"
			}
			fmt.Printf("[%s] %s (%dms)\n", status, res.Message, res.LatencyMS)
			if !res.OK {
				// Non-zero exit so scripts can detect an unhealthy provider.
				return fmt.Errorf("provider %d is unreachable", id)
			}
			return nil
		},
	}
}

func newProvidersHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Show the last health-check result for each provider",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := apiClient.get(cmd.Context(), "/api/providers/health", nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			var entries []struct {
				ProviderID   uint       `json:"provider_id"`
				ProviderName string     `json:"provider_name"`
				ProviderType string     `json:"provider_type"`
				LastTestedAt *time.Time `json:"last_tested_at"`
				LatencyMS    *int64     `json:"latency_ms"`
				Status       string     `json:"status"`
			}
			if err := json.Unmarshal(data, &entries); err != nil {
				return err
			}
			t := newTable("ID", "NAME", "TYPE", "STATUS", "LATENCY", "LAST TESTED")
			for _, e := range entries {
				lat := "-"
				if e.LatencyMS != nil {
					lat = fmt.Sprintf("%dms", *e.LatencyMS)
				}
				t.row(fmt.Sprint(e.ProviderID), e.ProviderName, e.ProviderType,
					e.Status, lat, fmtTime(e.LastTestedAt))
			}
			t.flush()
			return nil
		},
	}
}
