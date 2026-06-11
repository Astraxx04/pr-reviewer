package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newDashboardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Aggregate statistics",
	}
	cmd.AddCommand(newDashboardStatsCmd())
	return cmd
}

func newDashboardStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show review and repository summary statistics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := apiClient.get(cmd.Context(), "/api/dashboard/stats", nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			var s struct {
				TotalReviews   int64   `json:"total_reviews"`
				AvgScore       float64 `json:"avg_score"`
				Approvals      int64   `json:"approvals"`
				RequestChanges int64   `json:"request_changes"`
				Comments       int64   `json:"comments"`
				TotalRepos     int64   `json:"total_repos"`
				EnabledRepos   int64   `json:"enabled_repos"`
			}
			if err := json.Unmarshal(data, &s); err != nil {
				return err
			}
			t := newTable("METRIC", "VALUE")
			t.row("total reviews", fmt.Sprint(s.TotalReviews))
			t.row("avg score", fmt.Sprintf("%.1f", s.AvgScore))
			t.row("approvals", fmt.Sprint(s.Approvals))
			t.row("changes requested", fmt.Sprint(s.RequestChanges))
			t.row("comments", fmt.Sprint(s.Comments))
			t.row("repos (enabled/total)", fmt.Sprintf("%d/%d", s.EnabledRepos, s.TotalRepos))
			t.flush()
			return nil
		},
	}
}
