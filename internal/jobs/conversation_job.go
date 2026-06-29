package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/riverqueue/river"
	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/ai"
	"github.com/Astraxx04/pr-reviewer/internal/ai/mcp"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	gh "github.com/Astraxx04/pr-reviewer/internal/github"
	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

// ConversationJobArgs is the payload for a conversational re-review job.
type ConversationJobArgs struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	Number      int    `json:"number"`
	InReplyToID int64  `json:"in_reply_to_id"` // GitHub comment ID of the bot's original comment
	HumanBody   string `json:"human_body"`     // text of the human's reply
	BotBody     string `json:"bot_body"`       // text of the bot's original comment (for context)
}

func (ConversationJobArgs) Kind() string { return "conversation" }

// ConversationWorker handles replies to bot inline review comments.
type ConversationWorker struct {
	river.WorkerDefaults[ConversationJobArgs]

	GHClient     gh.Client
	DB           *gorm.DB
	Log          *logger.Logger
	Orchestrator *ai.AgentOrchestrator
}

func (w *ConversationWorker) Work(ctx context.Context, job *river.Job[ConversationJobArgs]) error {
	args := job.Args

	// Idempotency: skip if we already replied to this thread.
	var existing models.BotReply
	if w.DB.WithContext(ctx).Where("github_comment_id = ?", args.InReplyToID).First(&existing).Error == nil {
		w.Log.Info("already replied to comment thread, skipping", "comment_id", args.InReplyToID)
		return nil
	}

	prompt := fmt.Sprintf("Your original comment:\n%s\n\nHuman's reply:\n%s",
		args.BotBody, args.HumanBody)

	resp, err := w.Orchestrator.Dispatch(ctx, "conversation", mcp.Request{Query: prompt})
	if err != nil {
		w.Log.Error("conversation agent failed", "error", err)
		return err
	}

	var conv struct {
		Action string `json:"action"`
		Body   string `json:"body"`
	}
	text := ai.StripJSONFence(resp.Content)
	if err := json.Unmarshal([]byte(text), &conv); err != nil {
		return fmt.Errorf("conversation: parse agent response: %w", err)
	}

	if conv.Body == "" {
		return nil
	}

	if err := w.GHClient.PostReviewCommentReply(ctx, args.Owner, args.Repo, args.Number, args.InReplyToID, conv.Body); err != nil {
		w.Log.Error("failed to post conversation reply", "error", err)
		return err
	}

	w.DB.WithContext(ctx).Create(&models.BotReply{
		GithubCommentID: args.InReplyToID,
		RepliedAt:       time.Now(),
	})

	w.Log.Info("conversation reply posted", "comment_id", args.InReplyToID, "action", conv.Action)
	return nil
}
