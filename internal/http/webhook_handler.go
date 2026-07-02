package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"github.com/Astraxx04/pr-reviewer/internal/db/repo"
	"github.com/Astraxx04/pr-reviewer/internal/jobs"
	"github.com/Astraxx04/pr-reviewer/internal/metrics"
	"github.com/Astraxx04/pr-reviewer/internal/telemetry"
	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

type WebhookHandler struct {
	log           *logger.Logger
	deliveryRepo  *repo.DeliveryRepo
	enqueuer      jobs.JobEnqueuer
	webhookSecret []byte
	db            *gorm.DB // nil in tests; used for installation event handling
	allowedOrg    string   // if set, installations from any other account are ignored (single-org lock)
}

func NewWebhookHandler(
	log *logger.Logger,
	deliveryRepo *repo.DeliveryRepo,
	enqueuer jobs.JobEnqueuer,
	webhookSecret string,
) *WebhookHandler {
	return &WebhookHandler{
		log:           log,
		deliveryRepo:  deliveryRepo,
		enqueuer:      enqueuer,
		webhookSecret: []byte(webhookSecret),
	}
}

// WithDB attaches a database connection to the handler for installation event processing.
func (h *WebhookHandler) WithDB(db *gorm.DB) *WebhookHandler {
	h.db = db
	return h
}

// WithAllowedOrg restricts installation handling to a single GitHub account.
// When set, installation events from any other account are ignored, keeping the
// deployment locked to one organization. Empty means no restriction.
func (h *WebhookHandler) WithAllowedOrg(org string) *WebhookHandler {
	h.allowedOrg = org
	return h
}

func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.Tracer().Start(r.Context(), "webhook.receive")
	defer span.End()
	r = r.WithContext(ctx)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		metrics.WebhookRequestsTotal.WithLabelValues("error").Inc()
		return
	}

	if len(h.webhookSecret) > 0 {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !strings.HasPrefix(sig, "sha256=") {
			http.Error(w, "missing or invalid signature", http.StatusUnauthorized)
			metrics.WebhookRequestsTotal.WithLabelValues("invalid_sig").Inc()
			return
		}
		mac := hmac.New(sha256.New, h.webhookSecret)
		mac.Write(payload)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(sig), []byte(expected)) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			metrics.WebhookRequestsTotal.WithLabelValues("invalid_sig").Inc()
			return
		}
	}

	eventType := r.Header.Get("X-GitHub-Event")
	span.SetAttributes(attribute.String("webhook.event", eventType))

	switch eventType {
	case "pull_request", "": // empty header = legacy / tests
		h.handlePullRequest(r, payload, w)
	case "pull_request_review_comment":
		h.handlePRReviewComment(r.Context(), payload, w)
	case "issue_comment":
		h.handleIssueComment(r.Context(), payload, w)
	case "installation":
		h.handleInstallation(r.Context(), payload, w)
	case "installation_repositories":
		h.handleInstallationRepos(r.Context(), payload, w)
	case "organization":
		h.handleOrganization(r.Context(), payload, w)
	default:
		metrics.WebhookRequestsTotal.WithLabelValues("skipped").Inc()
		w.WriteHeader(http.StatusOK)
	}
}

// handlePullRequest processes pull_request webhook events.
func (h *WebhookHandler) handlePullRequest(r *http.Request, payload []byte, w http.ResponseWriter) {
	var event struct {
		Action      string `json:"action"`
		Number      int    `json:"number"`
		PullRequest struct {
			Draft bool `json:"draft"`
			Base  struct {
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

	h.log.Info("received webhook", "action", event.Action, "number", event.Number, "draft", event.PullRequest.Draft)

	validActions := map[string]bool{
		"opened":           true,
		"reopened":         true,
		"synchronize":      true,
		"ready_for_review": true,
	}
	if !validActions[event.Action] {
		metrics.WebhookRequestsTotal.WithLabelValues("skipped").Inc()
		w.WriteHeader(http.StatusOK)
		return
	}

	// Skip draft PRs unless they just became ready for review.
	if event.PullRequest.Draft && event.Action != "ready_for_review" {
		h.log.Info("skipping draft PR", "number", event.Number)
		metrics.WebhookRequestsTotal.WithLabelValues("draft_skipped").Inc()
		w.WriteHeader(http.StatusOK)
		return
	}

	owner := event.PullRequest.Base.Repo.Owner.Login
	repoName := event.PullRequest.Base.Repo.Name
	deliveryID := r.Header.Get("X-GitHub-Delivery")
	if deliveryID != "" && h.deliveryRepo != nil {
		if h.deliveryRepo.IsProcessed(r.Context(), deliveryID) {
			h.log.Info("duplicate delivery, skipping", "delivery_id", deliveryID)
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// Check per-repo review rate limit (default: 10 per hour, configurable via SystemConfig).
	if h.db != nil {
		maxPerHour := 10
		var cfgRow models.SystemConfig
		if h.db.Where("key = ?", "review_rate_limit_per_hour").First(&cfgRow).Error == nil {
			_, _ = fmt.Sscan(cfgRow.Value, &maxPerHour)
		}
		var recentCount int64
		h.db.Model(&models.Review{}).
			Joins("JOIN pull_requests ON pull_requests.id = reviews.pr_id").
			Joins("JOIN repositories ON repositories.id = pull_requests.repo_id").
			Where("repositories.owner = ? AND repositories.name = ? AND reviews.created_at >= ?",
				owner, repoName, time.Now().Add(-time.Hour)).
			Count(&recentCount)
		if int(recentCount) >= maxPerHour {
			h.log.Warn("per-repo review rate limit exceeded", "repo", owner+"/"+repoName)
			metrics.WebhookRequestsTotal.WithLabelValues("rate_limited").Inc()
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
	}

	insertOpts := &river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByState: []rivertype.JobState{rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateScheduled, rivertype.JobStateRetryable},
		},
	}
	if _, err := h.enqueuer.Insert(r.Context(), jobs.ReviewJobArgs{
		Owner:  owner,
		Repo:   repoName,
		Number: event.Number,
		Action: event.Action,
	}, insertOpts); err != nil {
		h.log.Error("failed to enqueue review job", "error", err)
		if deliveryID != "" && h.deliveryRepo != nil {
			_ = h.deliveryRepo.RecordDelivery(r.Context(), &models.WebhookDelivery{
				DeliveryID: deliveryID, EventType: "pull_request",
				Action: event.Action, Owner: owner, Repo: repoName,
				PRNumber: event.Number, Status: "failed",
			})
		}
		metrics.WebhookRequestsTotal.WithLabelValues("error").Inc()
		http.Error(w, "failed to enqueue job", http.StatusInternalServerError)
		return
	}

	if deliveryID != "" && h.deliveryRepo != nil {
		_ = h.deliveryRepo.RecordDelivery(r.Context(), &models.WebhookDelivery{
			DeliveryID: deliveryID, EventType: "pull_request",
			Action: event.Action, Owner: owner, Repo: repoName,
			PRNumber: event.Number, Status: "enqueued",
		})
	}
	metrics.WebhookRequestsTotal.WithLabelValues(event.Action).Inc()
	w.WriteHeader(http.StatusAccepted)
}

type ghInstallationPayload struct {
	Action       string `json:"action"`
	Installation struct {
		ID      int64 `json:"id"`
		Account struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"account"`
		Repositories []struct {
			FullName string `json:"full_name"`
		} `json:"repositories"`
	} `json:"installation"`
}

type ghInstallationReposPayload struct {
	Installation struct {
		ID      int64 `json:"id"`
		Account struct {
			Login string `json:"login"`
		} `json:"account"`
	} `json:"installation"`
	RepositoriesAdded []struct {
		FullName string `json:"full_name"`
	} `json:"repositories_added"`
	RepositoriesRemoved []struct {
		FullName string `json:"full_name"`
	} `json:"repositories_removed"`
}

// handleInstallation processes installation webhook events (created/deleted/suspend/unsuspend).
func (h *WebhookHandler) handleInstallation(ctx context.Context, payload []byte, w http.ResponseWriter) {
	var event ghInstallationPayload
	if err := json.Unmarshal(payload, &event); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	h.log.Info("installation event", "action", event.Action, "account", event.Installation.Account.Login)

	if h.db == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch event.Action {
	case "created":
		// Single-org lock: ignore installations from any account other than the
		// configured one. Defense-in-depth alongside the GitHub App's
		// "Only on this account" setting.
		if h.allowedOrg != "" && !strings.EqualFold(event.Installation.Account.Login, h.allowedOrg) {
			h.log.Warn("ignoring installation from non-allowed account",
				"account", event.Installation.Account.Login, "allowed", h.allowedOrg)
			break
		}
		// Reconcile on account_login (the natural key) so a stub installation
		// created by the review path before this webhook arrived is upgraded
		// in place with the real installation ID, rather than duplicated.
		instID := event.Installation.ID
		var inst models.Installation
		h.db.Where(models.Installation{AccountLogin: event.Installation.Account.Login}).
			Assign(models.Installation{
				GithubInstallationID: &instID,
				AccountType:          event.Installation.Account.Type,
			}).
			FirstOrCreate(&inst)

		// Upsert repos included in the installation payload.
		for _, r := range event.Installation.Repositories {
			owner, name := splitFullName(r.FullName)
			upsertRepo(ctx, h.db, owner, name)
		}

	case "deleted":
		var inst models.Installation
		if h.db.Where("github_installation_id = ?", event.Installation.ID).First(&inst).Error == nil {
			h.db.Delete(&inst)
		}
	}

	metrics.WebhookRequestsTotal.WithLabelValues("installation." + event.Action).Inc()
	w.WriteHeader(http.StatusOK)
}

// handleInstallationRepos processes installation_repositories events (repos added/removed).
func (h *WebhookHandler) handleInstallationRepos(ctx context.Context, payload []byte, w http.ResponseWriter) {
	var event ghInstallationReposPayload
	if err := json.Unmarshal(payload, &event); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if h.db == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	for _, r := range event.RepositoriesAdded {
		owner, name := splitFullName(r.FullName)
		upsertRepo(ctx, h.db, owner, name)
	}
	for _, r := range event.RepositoriesRemoved {
		owner, name := splitFullName(r.FullName)
		h.db.WithContext(ctx).
			Where("owner = ? AND name = ?", owner, name).
			Delete(&models.Repository{})
	}

	h.log.Info("repos synced via webhook",
		"added", len(event.RepositoriesAdded),
		"removed", len(event.RepositoriesRemoved),
	)
	w.WriteHeader(http.StatusOK)
}

// handleOrganization processes organization webhook events.
// A member_removed action immediately suspends the user and wipes their sessions.
func (h *WebhookHandler) handleOrganization(ctx context.Context, payload []byte, w http.ResponseWriter) {
	var event struct {
		Action     string `json:"action"`
		Membership struct {
			User struct {
				Login string `json:"login"`
			} `json:"user"`
		} `json:"membership"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	login := event.Membership.User.Login
	if event.Action != "member_removed" || login == "" || h.db == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	var user models.User
	if h.db.WithContext(ctx).Where("login = ?", login).First(&user).Error != nil {
		// Not a platform user — nothing to do.
		w.WriteHeader(http.StatusOK)
		return
	}

	h.db.WithContext(ctx).Model(&user).Update("status", "suspended")
	h.db.WithContext(ctx).Where("user_id = ?", user.ID).Delete(&models.Session{})

	h.log.Info("user suspended via org webhook", "login", login)
	metrics.WebhookRequestsTotal.WithLabelValues("org_member_removed").Inc()
	w.WriteHeader(http.StatusOK)
}

func upsertRepo(ctx context.Context, db *gorm.DB, owner, name string) {
	_, _, _ = repo.UpsertRepository(ctx, db, owner, name, false)
}

func splitFullName(fullName string) (owner, name string) {
	if i := strings.IndexByte(fullName, '/'); i >= 0 {
		return fullName[:i], fullName[i+1:]
	}
	return "", fullName
}

// handlePRReviewComment processes pull_request_review_comment events.
// When a human replies to a bot comment, a conversation job is enqueued.
func (h *WebhookHandler) handlePRReviewComment(ctx context.Context, payload []byte, w http.ResponseWriter) {
	var event struct {
		Action  string `json:"action"`
		Comment struct {
			ID          int64  `json:"id"`
			Body        string `json:"body"`
			InReplyToID int64  `json:"in_reply_to_id"`
			User        struct {
				Login string `json:"login"`
			} `json:"user"`
		} `json:"comment"`
		PullRequest struct {
			Number int `json:"number"`
			Base   struct {
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

	if event.Action != "created" || event.Comment.InReplyToID == 0 || h.db == nil || h.enqueuer == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Look up the original bot comment.
	var botComment models.BotComment
	if err := h.db.WithContext(ctx).Where("github_comment_id = ?", event.Comment.InReplyToID).First(&botComment).Error; err != nil {
		// Not a reply to a bot comment — ignore.
		w.WriteHeader(http.StatusOK)
		return
	}

	owner := event.PullRequest.Base.Repo.Owner.Login
	repo := event.PullRequest.Base.Repo.Name
	number := event.PullRequest.Number

	if _, err := h.enqueuer.Insert(ctx, jobs.ConversationJobArgs{
		Owner:       owner,
		Repo:        repo,
		Number:      number,
		InReplyToID: event.Comment.InReplyToID,
		HumanBody:   event.Comment.Body,
		BotBody:     botComment.Body,
	}, nil); err != nil {
		h.log.Error("failed to enqueue conversation job", "error", err)
		http.Error(w, "failed to enqueue job", http.StatusInternalServerError)
		return
	}

	h.log.Info("conversation job enqueued", "comment_id", event.Comment.InReplyToID, "pr", number)
	metrics.WebhookRequestsTotal.WithLabelValues("pr_review_comment").Inc()
	w.WriteHeader(http.StatusAccepted)
}

// handleIssueComment processes issue_comment events.
// A /re-review command in a PR comment triggers a new review job.
func (h *WebhookHandler) handleIssueComment(ctx context.Context, payload []byte, w http.ResponseWriter) {
	var event struct {
		Action string `json:"action"`
		Issue  struct {
			Number      int              `json:"number"`
			PullRequest *json.RawMessage `json:"pull_request"` // non-nil when this is a PR comment
		} `json:"issue"`
		Comment struct {
			Body string `json:"body"`
			User struct {
				Login string `json:"login"`
			} `json:"user"`
		} `json:"comment"`
		Repository struct {
			Owner struct {
				Login string `json:"login"`
			} `json:"owner"`
			Name string `json:"name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Only handle comments on PRs, not plain issues.
	if event.Action != "created" || event.Issue.PullRequest == nil || h.enqueuer == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	body := strings.TrimSpace(event.Comment.Body)
	if !strings.HasPrefix(body, "/re-review") {
		w.WriteHeader(http.StatusOK)
		return
	}

	owner := event.Repository.Owner.Login
	repo := event.Repository.Name
	number := event.Issue.Number

	insertOpts := &river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByState: []rivertype.JobState{rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateScheduled, rivertype.JobStateRetryable},
		},
	}
	if _, err := h.enqueuer.Insert(ctx, jobs.ReviewJobArgs{
		Owner:  owner,
		Repo:   repo,
		Number: number,
		Action: "re-review",
	}, insertOpts); err != nil {
		h.log.Error("failed to enqueue re-review job", "error", err)
		http.Error(w, "failed to enqueue job", http.StatusInternalServerError)
		return
	}

	h.log.Info("re-review job enqueued", "owner", owner, "repo", repo, "pr", number)
	metrics.WebhookRequestsTotal.WithLabelValues("re_review").Inc()
	w.WriteHeader(http.StatusAccepted)
}
