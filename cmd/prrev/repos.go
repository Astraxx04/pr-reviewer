package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type repoRow struct {
	ID             uint   `json:"id"`
	Owner          string `json:"owner"`
	Name           string `json:"name"`
	Enabled        bool   `json:"enabled"`
	IndexingStatus string `json:"indexingStatus"`
}

func newReposCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repos",
		Short: "List and manage tracked repositories",
	}
	cmd.AddCommand(
		newReposListCmd(),
		newReposEnableCmd(true),
		newReposEnableCmd(false),
		newReposSyncCmd(),
		newReposIndexCmd(),
	)
	return cmd
}

func newReposListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List repositories tracked by the server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := apiClient.get(cmd.Context(), "/api/repos", nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			var repos []repoRow
			if err := json.Unmarshal(data, &repos); err != nil {
				return err
			}
			t := newTable("ID", "REPO", "ENABLED", "INDEXING")
			for _, r := range repos {
				t.row(fmt.Sprint(r.ID), r.Owner+"/"+r.Name, yesNo(r.Enabled), orDash(r.IndexingStatus))
			}
			t.flush()
			return nil
		},
	}
}

// newReposEnableCmd builds either the `enable` or `disable` command.
func newReposEnableCmd(enable bool) *cobra.Command {
	verb := "disable"
	if enable {
		verb = "enable"
	}
	return &cobra.Command{
		Use:   verb + " <id>",
		Short: fmt.Sprintf("%s a repository for reviewing", verb),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			data, err := apiClient.patch(cmd.Context(),
				fmt.Sprintf("/api/repos/%d", id),
				map[string]any{"enabled": enable})
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			fmt.Printf("Repository %d %sd.\n", id, verb)
			return nil
		},
	}
}

func newReposSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync repositories from the GitHub App installation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := apiClient.post(cmd.Context(), "/api/repos/sync", nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			var res struct {
				Synced int `json:"synced"`
				Added  int `json:"added"`
			}
			if err := json.Unmarshal(data, &res); err != nil {
				return err
			}
			fmt.Printf("Synced %d repositories (%d newly added).\n", res.Synced, res.Added)
			return nil
		},
	}
}

func newReposIndexCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "index <id>",
		Short: "Trigger a full RAG re-index of a repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			data, err := apiClient.post(cmd.Context(), fmt.Sprintf("/api/repos/%d/index", id), nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			fmt.Printf("Indexing job enqueued for repository %d.\n", id)
			return nil
		},
	}
}
