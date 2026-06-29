package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/Astraxx04/pr-reviewer/internal/ai"
	gh "github.com/Astraxx04/pr-reviewer/internal/github"
	"github.com/Astraxx04/pr-reviewer/internal/pr"
	"github.com/Astraxx04/pr-reviewer/internal/review"
	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

// InProcessWebhookHandler is a fallback for local development when no database is configured.
// It runs the review pipeline in a goroutine — jobs are lost on process restart.
type InProcessWebhookHandler struct {
	log           *logger.Logger
	prService     pr.Service
	aiService     ai.Service
	aggregator    review.Aggregator
	ghClient      gh.Client
	webhookSecret []byte
}

func NewInProcessWebhookHandler(
	log *logger.Logger,
	prService pr.Service,
	aiService ai.Service,
	aggregator review.Aggregator,
	ghClient gh.Client,
	webhookSecret string,
) *InProcessWebhookHandler {
	return &InProcessWebhookHandler{
		log:           log,
		prService:     prService,
		aiService:     aiService,
		aggregator:    aggregator,
		ghClient:      ghClient,
		webhookSecret: []byte(webhookSecret),
	}
}

func (h *InProcessWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}

	if len(h.webhookSecret) > 0 {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !strings.HasPrefix(sig, "sha256=") {
			http.Error(w, "missing or invalid signature", http.StatusUnauthorized)
			return
		}
		mac := hmac.New(sha256.New, h.webhookSecret)
		mac.Write(payload)
		if !hmac.Equal([]byte(sig), []byte("sha256="+hex.EncodeToString(mac.Sum(nil)))) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var event struct {
		Action      string `json:"action"`
		Number      int    `json:"number"`
		PullRequest struct {
			Base struct {
				Repo struct {
					Owner struct {
						Login string `json:"login"`
					} `json:"owner"`
					Name string `json:"name"`
				} `json:"repo"`
			} `json:"base"`
		} `json:"pull_request"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if event.Action != "opened" && event.Action != "synchronize" {
		w.WriteHeader(http.StatusOK)
		return
	}

	owner := event.PullRequest.Base.Repo.Owner.Login
	repoName := event.PullRequest.Base.Repo.Name
	number := event.Number

	go func() {
		ctx := context.Background()
		prCtx, err := h.prService.BuildContext(ctx, owner, repoName, number, event.Action)
		if err != nil {
			h.log.Error("failed to build PR context", "error", err)
			return
		}
		result, err := h.aiService.Review(ctx, ai.AnalysisRequest{
			Diff: prCtx.Diff, Title: prCtx.Title, Body: prCtx.Body,
		})
		if err != nil {
			h.log.Error("AI review failed", "error", err)
			return
		}
		finalReview, err := h.aggregator.Aggregate(ctx, []ai.ReviewResult{*result})
		if err != nil {
			h.log.Error("aggregation failed", "error", err)
			return
		}
		if _, err := h.ghClient.PostReview(ctx, owner, repoName, number, &gh.ReviewSubmission{
			Body: finalReview.Summary, Event: finalReview.Status, Comments: finalReview.Comments,
		}); err != nil {
			h.log.Error("failed to post review", "error", err)
		}
	}()

	w.WriteHeader(http.StatusAccepted)
}
