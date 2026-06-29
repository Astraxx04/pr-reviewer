package assignments

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"github.com/Astraxx04/pr-reviewer/internal/db/repo"
	gh "github.com/Astraxx04/pr-reviewer/internal/github"
	"github.com/Astraxx04/pr-reviewer/internal/pr"
)

type Result struct {
	Assignees []string
}

type ruleConfig struct {
	Members []string `json:"members"`
}

// Evaluate runs the assignment rule strategy and returns who should review the PR.
func Evaluate(
	ctx context.Context,
	db *gorm.DB,
	rule *models.AssignmentRule,
	prCtx *pr.PRContext,
	codeowners []gh.CODEOWNERSRule,
	installationID uint,
) (*Result, error) {
	switch rule.Strategy {
	case "codeowners":
		return evaluateCodeowners(codeowners, prCtx)
	case "round-robin":
		return evaluateRoundRobin(ctx, db, rule, installationID)
	case "load-balanced":
		return evaluateLoadBalanced(ctx, db, rule, installationID)
	default:
		return &Result{}, nil
	}
}

func evaluateCodeowners(rules []gh.CODEOWNERSRule, prCtx *pr.PRContext) (*Result, error) {
	if len(rules) == 0 {
		return &Result{}, nil
	}
	files := make([]string, 0, len(prCtx.Diff))
	for _, f := range prCtx.Diff {
		files = append(files, f.Filename)
	}
	return &Result{Assignees: gh.MatchOwners(rules, files)}, nil
}

func evaluateRoundRobin(ctx context.Context, db *gorm.DB, rule *models.AssignmentRule, installationID uint) (*Result, error) {
	members, err := getMembers(ctx, db, rule, installationID)
	if err != nil || len(members) == 0 {
		return &Result{}, err
	}
	// Single-org assumption: the assignment count is intentionally not scoped by
	// installation. This deployment serves one organization (one installation),
	// so the global count equals this org's count. If multiple installations are
	// ever supported, scope this by installation_id.
	var count int64
	db.WithContext(ctx).Model(&models.Assignment{}).Count(&count)
	return &Result{Assignees: []string{members[int(count)%len(members)]}}, nil
}

func evaluateLoadBalanced(ctx context.Context, db *gorm.DB, rule *models.AssignmentRule, installationID uint) (*Result, error) {
	members, err := getMembers(ctx, db, rule, installationID)
	if err != nil || len(members) == 0 {
		return &Result{}, err
	}

	// Single-org assumption: the per-member tally is intentionally not scoped by
	// installation (one organization = one installation). Scope by installation_id
	// here if multiple installations are ever supported.
	type countRow struct {
		AssigneeLogin string
		Cnt           int64
	}
	var rows []countRow
	db.WithContext(ctx).Model(&models.Assignment{}).
		Select("assignee_login, count(*) as cnt").
		Group("assignee_login").
		Scan(&rows)

	tally := make(map[string]int64, len(rows))
	for _, r := range rows {
		tally[r.AssigneeLogin] = r.Cnt
	}

	pick := members[0]
	min := tally[members[0]]
	for _, m := range members[1:] {
		if tally[m] < min {
			min = tally[m]
			pick = m
		}
	}
	return &Result{Assignees: []string{pick}}, nil
}

func getMembers(ctx context.Context, db *gorm.DB, rule *models.AssignmentRule, installationID uint) ([]string, error) {
	var cfg ruleConfig
	if len(rule.Config) > 0 {
		_ = json.Unmarshal(rule.Config, &cfg)
	}
	if len(cfg.Members) > 0 {
		return cfg.Members, nil
	}
	teamMembers, err := repo.NewTeamMemberRepo(db).List(ctx, installationID)
	if err != nil {
		return nil, err
	}
	logins := make([]string, 0, len(teamMembers))
	for _, m := range teamMembers {
		logins = append(logins, m.Login)
	}
	return logins, nil
}
