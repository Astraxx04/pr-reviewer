// seed inserts idempotent demo data into the PR Reviewer database.
// It is safe to run multiple times — existing records are left unchanged.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/datatypes"

	"github.com/Astraxx04/pr-reviewer/internal/db"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
)

func main() {
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "seed: DATABASE_URL is not set")
		os.Exit(1)
	}

	gormDB, err := db.Connect(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed: connect: %v\n", err)
		os.Exit(1)
	}
	if err := db.RunMigrations(dsn); err != nil {
		fmt.Fprintf(os.Stderr, "seed: migrate: %v\n", err)
		os.Exit(1)
	}

	// ------------------------------------------------------------------ //
	// 1. Installation
	// ------------------------------------------------------------------ //
	instID := int64(1001)
	inst := models.Installation{
		GithubInstallationID: &instID,
		AccountLogin:         "demo-org",
		AccountType:          "Organization",
	}
	gormDB.Where(models.Installation{AccountLogin: "demo-org"}).FirstOrCreate(&inst)

	// ------------------------------------------------------------------ //
	// 2. Users (login identities) — keyed on GithubID
	// ------------------------------------------------------------------ //
	// These are the accounts you can actually authenticate as locally;
	// TeamMember rows below are the review-assignment roster, not login users.
	users := []models.User{
		{GithubID: 9001, Login: "alice", Email: "alice@demo-org.test", AvatarURL: "https://avatars.githubusercontent.com/u/9001", Role: "owner", Status: "active"},
		{GithubID: 9002, Login: "bob", Email: "bob@demo-org.test", AvatarURL: "https://avatars.githubusercontent.com/u/9002", Role: "reviewer", Status: "active"},
		{GithubID: 9003, Login: "carol", Email: "carol@demo-org.test", AvatarURL: "https://avatars.githubusercontent.com/u/9003", Role: "reviewer", Status: "active"},
	}
	for i := range users {
		gormDB.Where(models.User{GithubID: users[i].GithubID}).FirstOrCreate(&users[i])
	}
	aliceUser := users[0]

	// ------------------------------------------------------------------ //
	// 3. Repositories
	// ------------------------------------------------------------------ //
	apiAgentConfig := datatypes.JSON([]byte(`{"agents":{"code-review":{"provider_id":"1","model":"gpt-4o"}}}`))
	repoSpecs := []models.Repository{
		{InstallationID: inst.ID, Owner: "demo-org", Name: "api-service", Enabled: true, IndexingStatus: "indexed", Config: apiAgentConfig},
		{InstallationID: inst.ID, Owner: "demo-org", Name: "web-frontend", Enabled: true, IndexingStatus: "indexed"},
		{InstallationID: inst.ID, Owner: "demo-org", Name: "infra-tools", Enabled: false},
	}
	for i := range repoSpecs {
		gormDB.Where(models.Repository{
			InstallationID: inst.ID,
			Owner:          repoSpecs[i].Owner,
			Name:           repoSpecs[i].Name,
		}).FirstOrCreate(&repoSpecs[i])
	}
	apiServiceRepo := repoSpecs[0]
	webFrontendRepo := repoSpecs[1]

	// AssignmentRule for api-service (round-robin across the roster)
	ruleConfig := datatypes.JSON([]byte(`{"members":["alice","bob","carol"]}`))
	rule := models.AssignmentRule{RepoID: apiServiceRepo.ID, Strategy: "round-robin", Config: ruleConfig}
	gormDB.Where(models.AssignmentRule{RepoID: apiServiceRepo.ID, Strategy: "round-robin"}).FirstOrCreate(&rule)

	// ------------------------------------------------------------------ //
	// 4. Team members
	// ------------------------------------------------------------------ //
	members := []models.TeamMember{
		{InstallationID: inst.ID, Login: "alice", Role: "admin"},
		{InstallationID: inst.ID, Login: "bob", Role: "reviewer"},
		{InstallationID: inst.ID, Login: "carol", Role: "reviewer"},
	}
	for i := range members {
		gormDB.Where(models.TeamMember{
			InstallationID: inst.ID,
			Login:          members[i].Login,
		}).FirstOrCreate(&members[i])
	}

	// ------------------------------------------------------------------ //
	// 5. PRs + reviews for api-service
	// ------------------------------------------------------------------ //
	totalReviews := 0
	totalComments := 0
	totalFeedback := 0

	// upsertComment creates a comment if absent (keyed on review+path+line) and
	// always leaves the passed struct populated with its ID for downstream use.
	upsertComment := func(c *models.ReviewComment) {
		res := gormDB.Where(models.ReviewComment{
			ReviewID: c.ReviewID,
			Path:     c.Path,
			Line:     c.Line,
		}).FirstOrCreate(c)
		if res.RowsAffected > 0 {
			totalComments++
		}
	}

	// upsertFeedback records a vote on a comment, keyed on comment+user.
	upsertFeedback := func(commentID uint, user string, vote int) {
		fb := models.CommentFeedback{ReviewCommentID: commentID, UserLogin: user, Vote: vote}
		res := gormDB.Where(models.CommentFeedback{ReviewCommentID: commentID, UserLogin: user}).FirstOrCreate(&fb)
		if res.RowsAffected > 0 {
			totalFeedback++
		}
	}

	// PR #1
	pr1 := models.PullRequest{
		RepoID:  apiServiceRepo.ID,
		Number:  1,
		Title:   "feat: add user authentication",
		Author:  "bob",
		HeadSHA: "abc123",
	}
	gormDB.Where(models.PullRequest{RepoID: apiServiceRepo.ID, Number: 1}).FirstOrCreate(&pr1)

	// Review 1 for PR #1
	rev1 := models.Review{
		PRID:         pr1.ID,
		Status:       "REQUEST_CHANGES",
		Score:        42,
		Summary:      "Missing input validation on login endpoint. SQL injection risk in user query.",
		InputTokens:  12400,
		OutputTokens: 1850,
		TokenUsage:   14250,
		LatencyMS:    7200,
	}
	if err := gormDB.Where(models.Review{PRID: pr1.ID, Status: "REQUEST_CHANGES", Score: 42}).
		FirstOrCreate(&rev1).Error; err == nil {
		totalReviews++

		c0 := models.ReviewComment{
			ReviewID: rev1.ID,
			Path:     "internal/auth/handler.go",
			Line:     45,
			Side:     "RIGHT",
			Body:     "Direct string interpolation in SQL query — use parameterized queries",
			Severity: "high",
			Priority: "p0",
		}
		c1 := models.ReviewComment{
			ReviewID: rev1.ID,
			Path:     "internal/auth/handler.go",
			Line:     78,
			Body:     "No rate limiting on login attempts",
			Severity: "medium",
			Priority: "p1",
		}
		upsertComment(&c0)
		upsertComment(&c1)

		// Feedback: the SQLi catch was useful; the rate-limit note got a downvote.
		upsertFeedback(c0.ID, "bob", 1)
		upsertFeedback(c1.ID, "carol", -1)

		assign1 := models.Assignment{ReviewID: rev1.ID, AssigneeLogin: "alice"}
		gormDB.Where(models.Assignment{ReviewID: rev1.ID, AssigneeLogin: "alice"}).FirstOrCreate(&assign1)
	}

	// Review 2 for PR #1
	rev1b := models.Review{
		PRID:         pr1.ID,
		Status:       "APPROVE",
		Score:        88,
		Summary:      "Auth handler looks good after refactor. Added rate limiting and parameterized queries.",
		InputTokens:  9800,
		OutputTokens: 920,
		TokenUsage:   10720,
		LatencyMS:    4100,
	}
	if err := gormDB.Where(models.Review{PRID: pr1.ID, Status: "APPROVE", Score: 88}).
		FirstOrCreate(&rev1b).Error; err == nil {
		totalReviews++
	}

	// PR #2
	pr2 := models.PullRequest{
		RepoID:  apiServiceRepo.ID,
		Number:  2,
		Title:   "fix: resolve memory leak in connection pool",
		Author:  "carol",
		HeadSHA: "def456",
	}
	gormDB.Where(models.PullRequest{RepoID: apiServiceRepo.ID, Number: 2}).FirstOrCreate(&pr2)

	rev2 := models.Review{
		PRID:         pr2.ID,
		Status:       "COMMENT",
		Score:        71,
		Summary:      "Connection pool cleanup improved. One minor concern about goroutine leak.",
		InputTokens:  8600,
		OutputTokens: 1100,
		TokenUsage:   9700,
		LatencyMS:    5300,
	}
	if err := gormDB.Where(models.Review{PRID: pr2.ID, Status: "COMMENT", Score: 71}).
		FirstOrCreate(&rev2).Error; err == nil {
		totalReviews++

		c := models.ReviewComment{
			ReviewID: rev2.ID,
			Path:     "pkg/database/pool.go",
			Line:     112,
			Body:     "Goroutine started here may not be properly terminated on context cancellation",
			Severity: "medium",
			Priority: "p1",
		}
		upsertComment(&c)
		upsertFeedback(c.ID, "alice", 1)
	}

	// PR #3
	pr3 := models.PullRequest{
		RepoID:  apiServiceRepo.ID,
		Number:  3,
		Title:   "chore: update dependencies",
		Author:  "alice",
		HeadSHA: "ghi789",
	}
	gormDB.Where(models.PullRequest{RepoID: apiServiceRepo.ID, Number: 3}).FirstOrCreate(&pr3)

	rev3 := models.Review{
		PRID:         pr3.ID,
		Status:       "APPROVE",
		Score:        95,
		Summary:      "Dependency updates are clean. No breaking changes detected.",
		InputTokens:  4200,
		OutputTokens: 380,
		TokenUsage:   4580,
		LatencyMS:    2600,
	}
	if err := gormDB.Where(models.Review{PRID: pr3.ID, Status: "APPROVE", Score: 95}).
		FirstOrCreate(&rev3).Error; err == nil {
		totalReviews++
	}

	// ------------------------------------------------------------------ //
	// 6. PRs + reviews for web-frontend
	// ------------------------------------------------------------------ //

	// PR #4
	pr4 := models.PullRequest{
		RepoID:  webFrontendRepo.ID,
		Number:  4,
		Title:   "feat: dashboard redesign",
		Author:  "bob",
		HeadSHA: "jkl012",
	}
	gormDB.Where(models.PullRequest{RepoID: webFrontendRepo.ID, Number: 4}).FirstOrCreate(&pr4)

	rev4 := models.Review{
		PRID:         pr4.ID,
		Status:       "REQUEST_CHANGES",
		Score:        58,
		Summary:      "UI looks great but missing accessibility attributes and has XSS vulnerability.",
		InputTokens:  15600,
		OutputTokens: 2100,
		TokenUsage:   17700,
		LatencyMS:    8900,
	}
	if err := gormDB.Where(models.Review{PRID: pr4.ID, Status: "REQUEST_CHANGES", Score: 58}).
		FirstOrCreate(&rev4).Error; err == nil {
		totalReviews++

		c0 := models.ReviewComment{
			ReviewID: rev4.ID,
			Path:     "src/components/Dashboard.tsx",
			Line:     34,
			Body:     "innerHTML usage without sanitization — XSS risk",
			Severity: "critical",
			Priority: "p0",
		}
		c1 := models.ReviewComment{
			ReviewID: rev4.ID,
			Path:     "src/components/Dashboard.tsx",
			Line:     89,
			Body:     "Missing aria-label on interactive button",
			Severity: "low",
			Priority: "p3",
		}
		upsertComment(&c0)
		upsertComment(&c1)
		upsertFeedback(c0.ID, "carol", 1)
	}

	// PR #5
	pr5 := models.PullRequest{
		RepoID:  webFrontendRepo.ID,
		Number:  5,
		Title:   "fix: mobile responsive layout",
		Author:  "carol",
		HeadSHA: "mno345",
	}
	gormDB.Where(models.PullRequest{RepoID: webFrontendRepo.ID, Number: 5}).FirstOrCreate(&pr5)

	rev5 := models.Review{
		PRID:         pr5.ID,
		Status:       "APPROVE",
		Score:        82,
		Summary:      "Responsive fixes look solid. Good use of CSS Grid.",
		InputTokens:  7300,
		OutputTokens: 760,
		TokenUsage:   8060,
		LatencyMS:    3800,
	}
	if err := gormDB.Where(models.Review{PRID: pr5.ID, Status: "APPROVE", Score: 82}).
		FirstOrCreate(&rev5).Error; err == nil {
		totalReviews++
	}

	// ------------------------------------------------------------------ //
	// 7. ProviderConfig + health
	// ------------------------------------------------------------------ //
	provider := models.ProviderConfig{
		InstallationID:     inst.ID,
		Name:               "OpenAI GPT-4o",
		Type:               "openai",
		DefaultModel:       "gpt-4o",
		SupportsEmbeddings: false,
	}
	gormDB.Where(models.ProviderConfig{InstallationID: inst.ID, Name: "OpenAI GPT-4o"}).
		FirstOrCreate(&provider)

	health := models.ProviderHealth{
		ProviderConfigID: provider.ID,
		LatencyMS:        430,
		OK:               true,
	}
	gormDB.Where(models.ProviderHealth{ProviderConfigID: provider.ID}).FirstOrCreate(&health)

	// ------------------------------------------------------------------ //
	// 8. NotificationConfig
	// ------------------------------------------------------------------ //
	notifCfgJSON, _ := json.Marshal(map[string]any{
		"webhook_url":     "https://hooks.slack.com/demo",
		"events":          []string{"assignment", "review_complete"},
		"score_threshold": 0,
		"template":        "",
	})
	notif := models.NotificationConfig{
		InstallationID: inst.ID,
		Channel:        "slack",
		Config:         notifCfgJSON,
		Enabled:        true,
	}
	gormDB.Where(models.NotificationConfig{InstallationID: inst.ID, Channel: "slack"}).
		FirstOrCreate(&notif)

	// ------------------------------------------------------------------ //
	// 9. Audit log (admin actions for the compliance view)
	// ------------------------------------------------------------------ //
	auditEntries := []models.AuditLog{
		{
			ActorLogin: "alice",
			ActorID:    aliceUser.ID,
			Action:     "repo.enable",
			EntityType: "repo",
			EntityID:   fmt.Sprint(apiServiceRepo.ID),
			After:      datatypes.JSON([]byte(`{"enabled":true}`)),
			IPAddress:  "203.0.113.10",
		},
		{
			ActorLogin: "alice",
			ActorID:    aliceUser.ID,
			Action:     "provider.create",
			EntityType: "provider",
			EntityID:   fmt.Sprint(provider.ID),
			After:      datatypes.JSON([]byte(`{"name":"OpenAI GPT-4o","type":"openai"}`)),
			IPAddress:  "203.0.113.10",
		},
	}
	for i := range auditEntries {
		gormDB.Where(models.AuditLog{
			ActorLogin: auditEntries[i].ActorLogin,
			Action:     auditEntries[i].Action,
			EntityID:   auditEntries[i].EntityID,
		}).FirstOrCreate(&auditEntries[i])
	}

	// ------------------------------------------------------------------ //
	// Summary
	// ------------------------------------------------------------------ //
	fmt.Printf(
		"Seeded: 1 installation, 3 users, 3 repos, 3 team members, 5 PRs, %d reviews, %d comments, %d feedback votes\n",
		totalReviews,
		totalComments,
		totalFeedback,
	)
}
