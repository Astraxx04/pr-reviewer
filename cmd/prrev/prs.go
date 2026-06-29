package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"
)

func newPRsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prs",
		Short: "Inspect pull requests and trigger re-reviews",
	}
	cmd.AddCommand(newPRsListCmd(), newPRsGetCmd(), newPRsDiffCmd(), newPRsReReviewCmd())
	return cmd
}

type prSummary struct {
	Number         int        `json:"number"`
	Title          string     `json:"title"`
	Author         string     `json:"author"`
	Repo           string     `json:"repo"`
	PRStatus       string     `json:"pr_status"`
	CurrentScore   int        `json:"current_score"`
	ReviewCount    int64      `json:"review_count"`
	LastReviewedAt *time.Time `json:"last_reviewed_at"`
}

func newPRsListCmd() *cobra.Command {
	var repo, author, status string
	var page, perPage int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pull requests",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			q := url.Values{}
			if repo != "" {
				q.Set("repo", repo)
			}
			if author != "" {
				q.Set("author", author)
			}
			if status != "" {
				q.Set("status", status)
			}
			q.Set("page", fmt.Sprint(page))
			q.Set("per_page", fmt.Sprint(perPage))

			data, err := apiClient.get(cmd.Context(), "/api/prs", q)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			var res struct {
				PRs   []prSummary `json:"prs"`
				Total int64       `json:"total"`
			}
			if err := json.Unmarshal(data, &res); err != nil {
				return err
			}
			t := newTable("PR", "REPO", "TITLE", "STATUS", "SCORE", "REVIEWS", "AUTHOR")
			for _, p := range res.PRs {
				t.row(
					fmt.Sprintf("#%d", p.Number),
					p.Repo,
					truncate(p.Title, 50),
					p.PRStatus,
					fmt.Sprint(p.CurrentScore),
					fmt.Sprint(p.ReviewCount),
					p.Author,
				)
			}
			t.flush()
			fmt.Printf("\n%d total\n", res.Total)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&repo, "repo", "", "filter by repo (owner/name)")
	f.StringVar(&author, "author", "", "filter by author login")
	f.StringVar(&status, "status", "", "filter by status (approved|changes_requested|commented|pending)")
	f.IntVar(&page, "page", 1, "page number")
	f.IntVar(&perPage, "per-page", 20, "results per page (max 100)")
	return cmd
}

type prDetail struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	Author    string   `json:"author"`
	Repo      string   `json:"repo"`
	HeadSHA   string   `json:"head_sha"`
	PRStatus  string   `json:"pr_status"`
	Assignees []string `json:"assignees"`
	Reviews   []struct {
		ID           uint      `json:"id"`
		Status       string    `json:"status"`
		Score        int       `json:"score"`
		Summary      string    `json:"summary"`
		CommentCount int       `json:"comment_count"`
		CreatedAt    time.Time `json:"created_at"`
	} `json:"reviews"`
}

func newPRsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <owner/repo#N>",
		Short: "Show a pull request and its review history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, repo, number, err := parsePRRef(args[0])
			if err != nil {
				return err
			}
			data, err := apiClient.get(cmd.Context(),
				fmt.Sprintf("/api/prs/%s/%s/%d", owner, repo, number), nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			var pr prDetail
			if err := json.Unmarshal(data, &pr); err != nil {
				return err
			}
			fmt.Printf("%s#%d  %s\n", pr.Repo, pr.Number, pr.Title)
			fmt.Printf("Author : %s\n", pr.Author)
			fmt.Printf("Status : %s\n", pr.PRStatus)
			if len(pr.Assignees) > 0 {
				fmt.Printf("Assigned: %v\n", pr.Assignees)
			}
			if len(pr.Reviews) == 0 {
				fmt.Println("\nNo reviews yet.")
				return nil
			}
			fmt.Println("\nReviews (oldest first):")
			t := newTable("ID", "STATUS", "SCORE", "COMMENTS", "WHEN")
			for _, rv := range pr.Reviews {
				t.row(fmt.Sprint(rv.ID), rv.Status, fmt.Sprint(rv.Score),
					fmt.Sprint(rv.CommentCount), rv.CreatedAt.Format("2006-01-02 15:04"))
			}
			t.flush()
			// The latest review is the last element (server orders ascending).
			latest := pr.Reviews[len(pr.Reviews)-1]
			fmt.Printf("\nLatest summary: %s\n", latest.Summary)
			return nil
		},
	}
}

func newPRsDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <owner/repo#N>",
		Short: "Print the PR's unified diff (structured JSON)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, repo, number, err := parsePRRef(args[0])
			if err != nil {
				return err
			}
			data, err := apiClient.get(cmd.Context(),
				fmt.Sprintf("/api/prs/%s/%s/%d/diff", owner, repo, number), nil)
			if err != nil {
				return err
			}
			// The diff is structured per-file JSON; always print it as JSON.
			return printJSON(data)
		},
	}
}

func newPRsReReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "re-review <owner/repo#N>",
		Aliases: []string{"review"},
		Short:   "Trigger a re-review of a pull request",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, repo, number, err := parsePRRef(args[0])
			if err != nil {
				return err
			}
			data, err := apiClient.post(cmd.Context(),
				fmt.Sprintf("/api/prs/%s/%s/%d/re-review", owner, repo, number), nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			fmt.Printf("Re-review enqueued for %s/%s#%d.\n", owner, repo, number)
			return nil
		},
	}
}
