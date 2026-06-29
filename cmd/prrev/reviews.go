package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"
)

func newReviewsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reviews",
		Short: "List, inspect, and export reviews",
	}
	cmd.AddCommand(newReviewsListCmd(), newReviewsGetCmd(), newReviewsExportCmd())
	return cmd
}

type reviewRow struct {
	ID         uint      `json:"ID"`
	Status     string    `json:"Status"`
	Score      int       `json:"Score"`
	TokenUsage int       `json:"TokenUsage"`
	LatencyMS  int64     `json:"LatencyMS"`
	CreatedAt  time.Time `json:"CreatedAt"`
	Comments   []struct {
		ID uint `json:"ID"`
	} `json:"Comments"`
}

func newReviewsListCmd() *cobra.Command {
	var page, perPage int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List reviews (most recent first)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			q := url.Values{}
			q.Set("page", fmt.Sprint(page))
			q.Set("per_page", fmt.Sprint(perPage))
			data, err := apiClient.get(cmd.Context(), "/api/reviews", q)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			var res struct {
				Reviews []reviewRow `json:"reviews"`
				Total   int64       `json:"total"`
			}
			if err := json.Unmarshal(data, &res); err != nil {
				return err
			}
			t := newTable("ID", "STATUS", "SCORE", "TOKENS", "LATENCY", "COMMENTS", "WHEN")
			for _, rv := range res.Reviews {
				t.row(fmt.Sprint(rv.ID), rv.Status, fmt.Sprint(rv.Score),
					fmt.Sprint(rv.TokenUsage), fmt.Sprintf("%dms", rv.LatencyMS),
					fmt.Sprint(len(rv.Comments)), rv.CreatedAt.Format("2006-01-02 15:04"))
			}
			t.flush()
			fmt.Printf("\n%d total\n", res.Total)
			return nil
		},
	}
	f := cmd.Flags()
	f.IntVar(&page, "page", 1, "page number")
	f.IntVar(&perPage, "per-page", 20, "results per page (max 100)")
	return cmd
}

func newReviewsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Show a single review with its comments",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			data, err := apiClient.get(cmd.Context(), fmt.Sprintf("/api/reviews/%d", id), nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			var rv struct {
				ID       uint   `json:"ID"`
				Status   string `json:"Status"`
				Score    int    `json:"Score"`
				Summary  string `json:"Summary"`
				Comments []struct {
					Path     string `json:"Path"`
					Line     int    `json:"Line"`
					Severity string `json:"Severity"`
					Priority string `json:"Priority"`
					Body     string `json:"Body"`
				} `json:"Comments"`
			}
			if err := json.Unmarshal(data, &rv); err != nil {
				return err
			}
			fmt.Printf("Review #%d  [%s]  score %d\n", rv.ID, rv.Status, rv.Score)
			fmt.Printf("Summary: %s\n", rv.Summary)
			if len(rv.Comments) > 0 {
				fmt.Println("\nComments:")
				t := newTable("SEVERITY", "PRIORITY", "LOCATION", "COMMENT")
				for _, c := range rv.Comments {
					t.row(orDash(c.Severity), orDash(c.Priority),
						fmt.Sprintf("%s:%d", c.Path, c.Line), truncate(c.Body, 60))
				}
				t.flush()
			}
			return nil
		},
	}
}

func newReviewsExportCmd() *cobra.Command {
	var start, end, repo, out string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export reviews as CSV (to stdout or --out file)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			q := url.Values{}
			if start != "" {
				q.Set("start", start)
			}
			if end != "" {
				q.Set("end", end)
			}
			if repo != "" {
				q.Set("repo", repo)
			}
			data, err := apiClient.get(cmd.Context(), "/api/reviews/export", q)
			if err != nil {
				return err
			}
			return writeOut(out, data)
		},
	}
	f := cmd.Flags()
	f.StringVar(&start, "start", "", "start date (YYYY-MM-DD)")
	f.StringVar(&end, "end", "", "end date (YYYY-MM-DD)")
	f.StringVar(&repo, "repo", "", "filter by repo (owner/name)")
	f.StringVar(&out, "out", "", "write CSV to this file instead of stdout")
	return cmd
}
