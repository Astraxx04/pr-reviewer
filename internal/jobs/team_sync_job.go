package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	gogithub "github.com/google/go-github/v69/github"
	"github.com/riverqueue/river"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"pr-reviewer/internal/db/models"
	"pr-reviewer/pkg/logger"
)

// TeamSyncJobArgs triggers a sync of GitHub team membership into the team_members table.
type TeamSyncJobArgs struct{}

func (TeamSyncJobArgs) Kind() string { return "team_sync" }

// teamSyncRule is one entry from the SystemConfig "team_sync_rules" JSON array.
type teamSyncRule struct {
	Org  string `json:"org"`
	Team string `json:"team"`
	Role string `json:"role"`
}

// TeamSyncWorker syncs GitHub team members into the local TeamMember table.
type TeamSyncWorker struct {
	river.WorkerDefaults[TeamSyncJobArgs]

	DB          *gorm.DB
	Log         *logger.Logger
	GitHubToken string // PAT with read:org scope
}

func (w *TeamSyncWorker) Work(ctx context.Context, job *river.Job[TeamSyncJobArgs]) error {
	// Load sync rules from SystemConfig.
	var cfg models.SystemConfig
	if err := w.DB.WithContext(ctx).Where("key = ?", "team_sync_rules").First(&cfg).Error; err != nil {
		w.Log.Info("team sync: no rules configured, skipping")
		return nil
	}
	var rules []teamSyncRule
	if err := json.Unmarshal([]byte(cfg.Value), &rules); err != nil || len(rules) == 0 {
		w.Log.Info("team sync: no valid rules, skipping")
		return nil
	}

	if w.GitHubToken == "" {
		return fmt.Errorf("team sync: GITHUB_TOKEN not set")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: w.GitHubToken})
	ghClient := gogithub.NewClient(oauth2.NewClient(ctx, ts))

	for _, rule := range rules {
		if err := w.syncTeam(ctx, ghClient, rule); err != nil {
			w.Log.Error("team sync: failed to sync team", "org", rule.Org, "team", rule.Team, "error", err)
			// Continue with next rule.
		}
	}
	return nil
}

func (w *TeamSyncWorker) syncTeam(ctx context.Context, ghClient *gogithub.Client, rule teamSyncRule) error {
	opts := &gogithub.TeamListTeamMembersOptions{ListOptions: gogithub.ListOptions{PerPage: 100}}
	var allMembers []*gogithub.User
	for {
		members, resp, err := ghClient.Teams.ListTeamMembersBySlug(ctx, rule.Org, rule.Team, opts)
		if err != nil {
			return fmt.Errorf("list team members %s/%s: %w", rule.Org, rule.Team, err)
		}
		allMembers = append(allMembers, members...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Find (or create) the installation for this org.
	var inst models.Installation
	if err := w.DB.WithContext(ctx).Where("account_login = ?", rule.Org).First(&inst).Error; err != nil {
		w.Log.Error("team sync: installation not found for org", "org", rule.Org)
		return nil
	}

	// Build a login→bool map for O(1) lookup.
	loginSet := make(map[string]bool, len(allMembers))
	for _, m := range allMembers {
		loginSet[m.GetLogin()] = true
	}

	// Upsert members that are in the team.
	now := time.Now()
	for login := range loginSet {
		var existing models.TeamMember
		res := w.DB.WithContext(ctx).
			Where("installation_id = ? AND login = ?", inst.ID, login).
			First(&existing)
		if res.Error != nil {
			// New member — insert.
			w.DB.WithContext(ctx).Create(&models.TeamMember{
				InstallationID: inst.ID,
				Login:          login,
				Role:           rule.Role,
				CreatedAt:      now,
			})
		} else if existing.Role != rule.Role {
			// Role changed — update.
			w.DB.WithContext(ctx).Model(&existing).Update("role", rule.Role)
		}
	}

	// Remove members no longer in the team (for this specific rule's org/team scope).
	// We only remove members whose role matches this rule — prevents conflicts when
	// a member belongs to multiple teams with different roles.
	var currentMembers []models.TeamMember
	w.DB.WithContext(ctx).
		Where("installation_id = ? AND role = ?", inst.ID, rule.Role).
		Find(&currentMembers)
	for _, m := range currentMembers {
		if !loginSet[m.Login] {
			w.DB.WithContext(ctx).Delete(&m)
		}
	}

	w.Log.Info("team sync: synced", "org", rule.Org, "team", rule.Team, "members", len(allMembers))
	return nil
}
