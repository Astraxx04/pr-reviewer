package pr

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"

	gh "github.com/Astraxx04/pr-reviewer/internal/github"
	"github.com/Astraxx04/pr-reviewer/internal/telemetry"
	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

// this defines the interface for PR operations.
type Service interface {
	BuildContext(ctx context.Context, owner, repo string, number int, action string) (*PRContext, error)
}

// this is the implementation of the Service interface.
type serviceImpl struct {
	ghClient gh.Client
	log      *logger.Logger
}

func NewService(ghClient gh.Client, log *logger.Logger) Service {
	return &serviceImpl{ghClient: ghClient, log: log}
}

func (s *serviceImpl) BuildContext(ctx context.Context, owner, repo string, number int, action string) (*PRContext, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "pr.build_context")
	defer span.End()
	span.SetAttributes(
		attribute.String("pr.owner", owner),
		attribute.String("pr.repo", repo),
		attribute.Int("pr.number", number),
	)
	s.log.Info("Building PR context", "owner", owner, "repo", repo, "number", number)

	pr, err := s.ghClient.GetPullRequest(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR: %w", err)
	}

	diff, err := s.ghClient.GetPullRequestDiff(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}

	ctxData := &PRContext{
		Repo:      fmt.Sprintf("%s/%s", owner, repo),
		Number:    number,
		Title:     pr.Title,
		Body:      pr.Body,
		Action:    action,
		Timestamp: pr.UpdatedAt,
		PR:        pr,
		Diff:      diff,
	}
	s.log.Info("PR Context Built", "repo", ctxData.Repo, "number", ctxData.Number, "action", ctxData.Action, "files", len(ctxData.Diff))
	return ctxData, nil
}
