package http

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/http/handlers"
	"github.com/Astraxx04/pr-reviewer/internal/http/middleware"
)

// WebhookHandlerIface is satisfied by WebhookHandler and InProcessWebhookHandler.
type WebhookHandlerIface interface {
	Handle(http.ResponseWriter, *http.Request)
}

type RouterConfig struct {
	WebhookHandler         WebhookHandlerIface
	AuthHandler            *handlers.AuthHandler
	RepoHandler            *handlers.RepoHandler
	ReviewHandler          *handlers.ReviewHandler
	DashHandler            *handlers.DashboardHandler
	TeamHandler            *handlers.TeamHandler
	AssignHandler          *handlers.AssignmentHandler
	AnalyticsHandler       *handlers.AnalyticsHandler
	ProviderHandler        *handlers.ProviderHandler
	HealthHandler          *handlers.HealthHandler
	SetupHandler           *handlers.SetupHandler
	GithubAppHandler       *handlers.GithubAppHandler
	UserHandler            *handlers.UserHandler
	SessionHandler         *handlers.SessionHandler
	SystemMetricsHandler   *handlers.SystemMetricsHandler
	WebhookDeliveryHandler *handlers.WebhookDeliveryHandler
	ExportHandler          *handlers.ExportHandler
	PRHandler              *handlers.PRHandler
	NotificationHandler    *handlers.NotificationHandler
	FeedbackHandler        *handlers.FeedbackHandler
	ExplainHandler         *handlers.ExplainHandler
	EventsHandler          *handlers.EventsHandler
	AuditHandler           *handlers.AuditHandler
	RetentionHandler       *handlers.RetentionHandler
	SSOHandler             *handlers.SSOHandler
	APITokenHandler        *handlers.APITokenHandler
	InviteHandler          *handlers.InviteHandler
	IntegrationHandler     *handlers.IntegrationHandler
	SlackAppHandler        *handlers.SlackAppHandler
	RateLimiter            *middleware.RateLimiter
	JWTSecret              string
	AllowedOrigin          string
	DB                     *gorm.DB // for session validation in auth middleware
}

func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	// Public routes.
	mux.HandleFunc("/webhooks", cfg.WebhookHandler.Handle)
	if cfg.HealthHandler != nil {
		mux.HandleFunc("/healthz", cfg.HealthHandler.Health)
	}
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())

	// Slack inbound endpoints — public; authenticated via Slack request signature.
	if cfg.SlackAppHandler != nil {
		mux.HandleFunc("POST /slack/commands", cfg.SlackAppHandler.HandleCommand)
		mux.HandleFunc("POST /slack/events", cfg.SlackAppHandler.HandleEvents)
	}

	// SSE stream — auth handled inside the handler via ?token= query param.
	if cfg.EventsHandler != nil {
		mux.HandleFunc("GET /api/events", cfg.EventsHandler.Subscribe)
	}

	// Auth routes (public).
	if cfg.AuthHandler != nil {
		mux.HandleFunc("GET /auth/github", cfg.AuthHandler.Login)
		mux.HandleFunc("GET /auth/github/callback", cfg.AuthHandler.Callback)
		mux.HandleFunc("GET /auth/github/consent", cfg.AuthHandler.ConsentPage)
		mux.HandleFunc("POST /auth/github/continue", cfg.AuthHandler.ContinueLogin)
	}

	// OIDC SSO routes (public — redirect-based flow).
	if cfg.SSOHandler != nil {
		mux.HandleFunc("GET /auth/oidc", cfg.SSOHandler.Login)
		mux.HandleFunc("GET /auth/oidc/callback", cfg.SSOHandler.Callback)
	}

	// Setup routes (public — needed before first login).
	if cfg.SetupHandler != nil {
		mux.HandleFunc("GET /api/setup/status", cfg.SetupHandler.Status)
		mux.HandleFunc("POST /api/setup/complete", cfg.SetupHandler.Complete)
		mux.HandleFunc("POST /api/setup/reset", cfg.SetupHandler.Reset)
	}

	// Public invite token validation — must be registered on the outer mux before
	// the /api/ catch-all so it takes precedence without requiring auth.
	if cfg.InviteHandler != nil {
		mux.HandleFunc("GET /api/invites/validate", cfg.InviteHandler.Validate)
	}

	// Protected API routes — wrap with JWT auth + session check.
	api := http.NewServeMux()

	// adminFunc registers a route that only owner/admin roles may call. The Auth
	// middleware (applied to the whole api mux below) populates the user in the
	// request context, so RequireRole can authorize here. Settings and management
	// endpoints use this; member-facing read routes use api.HandleFunc directly.
	adminOnly := middleware.RequireRole("owner", "admin")
	adminFunc := func(pattern string, fn http.HandlerFunc) {
		api.Handle(pattern, adminOnly(fn))
	}

	if cfg.AuthHandler != nil {
		api.HandleFunc("GET /api/auth/me", cfg.AuthHandler.Me)
		api.HandleFunc("POST /api/auth/logout", cfg.AuthHandler.Logout)
	}
	if cfg.UserHandler != nil {
		adminFunc("GET /api/users", cfg.UserHandler.List)
		adminFunc("PATCH /api/users/{id}/role", cfg.UserHandler.UpdateRole)
		adminFunc("PATCH /api/users/{id}/approve", cfg.UserHandler.Approve)
		adminFunc("PATCH /api/users/{id}/reject", cfg.UserHandler.Reject)
		adminFunc("DELETE /api/users/{id}", cfg.UserHandler.Remove)
	}
	if cfg.SessionHandler != nil {
		api.HandleFunc("GET /api/sessions", cfg.SessionHandler.List)
		api.HandleFunc("DELETE /api/sessions/{id}", cfg.SessionHandler.Revoke)
		api.HandleFunc("DELETE /api/sessions", cfg.SessionHandler.RevokeAll)
	}
	if cfg.RepoHandler != nil {
		api.HandleFunc("GET /api/repos", cfg.RepoHandler.List)
		api.HandleFunc("GET /api/repos/{id}/config", cfg.RepoHandler.GetConfig)
		// Mutations are admin-only.
		adminFunc("PATCH /api/repos/{id}", cfg.RepoHandler.Update)
		adminFunc("PUT /api/repos/{id}/config", cfg.RepoHandler.PutConfig)
		adminFunc("POST /api/repos/sync", cfg.RepoHandler.Sync)
		adminFunc("POST /api/repos/{id}/index", cfg.RepoHandler.Index)
	}
	if cfg.ReviewHandler != nil {
		api.HandleFunc("GET /api/reviews", cfg.ReviewHandler.List)
		api.HandleFunc("GET /api/reviews/{id}", cfg.ReviewHandler.Get)
	}
	if cfg.DashHandler != nil {
		api.HandleFunc("GET /api/dashboard/stats", cfg.DashHandler.Stats)
	}
	if cfg.TeamHandler != nil {
		api.HandleFunc("GET /api/team", cfg.TeamHandler.List)
	}
	if cfg.InviteHandler != nil {
		adminFunc("POST /api/invites", cfg.InviteHandler.Create)
		adminFunc("POST /api/invites/bulk", cfg.InviteHandler.Bulk)
		adminFunc("GET /api/invites", cfg.InviteHandler.List)
		adminFunc("DELETE /api/invites/{id}", cfg.InviteHandler.Delete)
		adminFunc("POST /api/invites/{id}/resend", cfg.InviteHandler.Resend)
	}
	if cfg.AssignHandler != nil {
		api.HandleFunc("GET /api/repos/{repo_id}/assignments/rules", cfg.AssignHandler.ListRules)
		adminFunc("POST /api/repos/{repo_id}/assignments/rules", cfg.AssignHandler.CreateRule)
	}
	if cfg.AnalyticsHandler != nil {
		api.HandleFunc("GET /api/analytics", cfg.AnalyticsHandler.Analytics)
	}
	if cfg.ProviderHandler != nil {
		adminFunc("GET /api/providers", cfg.ProviderHandler.List)
		adminFunc("POST /api/providers", cfg.ProviderHandler.Create)
		adminFunc("POST /api/providers/models", cfg.ProviderHandler.ListModels)
		adminFunc("PUT /api/providers/{id}", cfg.ProviderHandler.Update)
		adminFunc("DELETE /api/providers/{id}", cfg.ProviderHandler.Delete)
		adminFunc("POST /api/providers/{id}/test", cfg.ProviderHandler.Test)
	}
	if cfg.GithubAppHandler != nil {
		adminFunc("GET /api/settings/github-app", cfg.GithubAppHandler.Get)
		adminFunc("PUT /api/settings/github-app", cfg.GithubAppHandler.Put)
		adminFunc("DELETE /api/settings/github-app", cfg.GithubAppHandler.Delete)
		adminFunc("POST /api/settings/github-app/test", cfg.GithubAppHandler.Test)
	}
	if cfg.SystemMetricsHandler != nil {
		adminFunc("GET /api/metrics/system", cfg.SystemMetricsHandler.Metrics)
	}
	if cfg.AnalyticsHandler != nil {
		adminFunc("GET /api/analytics/cost", cfg.AnalyticsHandler.Cost)
	}
	if cfg.ProviderHandler != nil {
		adminFunc("GET /api/providers/health", cfg.ProviderHandler.Health)
	}
	if cfg.WebhookDeliveryHandler != nil {
		adminFunc("GET /api/webhooks/deliveries", cfg.WebhookDeliveryHandler.List)
	}
	if cfg.ExportHandler != nil {
		api.HandleFunc("GET /api/reviews/export", cfg.ExportHandler.ReviewsCSV)
		api.HandleFunc("GET /api/reviews/export.pdf", cfg.ExportHandler.ReviewsPDF)
	}
	if cfg.PRHandler != nil {
		api.HandleFunc("GET /api/prs", cfg.PRHandler.List)
		api.HandleFunc("GET /api/prs/{owner}/{repo}/{number}", cfg.PRHandler.Get)
		api.HandleFunc("GET /api/prs/{owner}/{repo}/{number}/diff", cfg.PRHandler.Diff)
		api.HandleFunc("POST /api/prs/{owner}/{repo}/{number}/re-review", cfg.PRHandler.ReReview)
	}
	if cfg.NotificationHandler != nil {
		adminFunc("GET /api/settings/notifications", cfg.NotificationHandler.List)
		adminFunc("POST /api/settings/notifications", cfg.NotificationHandler.Create)
		adminFunc("PUT /api/settings/notifications/{id}", cfg.NotificationHandler.Update)
		adminFunc("DELETE /api/settings/notifications/{id}", cfg.NotificationHandler.Delete)
		adminFunc("POST /api/settings/notifications/{id}/test", cfg.NotificationHandler.Test)
		adminFunc("POST /api/settings/notifications/digest/trigger", cfg.NotificationHandler.TriggerDigest)
	}
	if cfg.FeedbackHandler != nil {
		api.HandleFunc("GET /api/reviews/comments/{id}/feedback", cfg.FeedbackHandler.Get)
		api.HandleFunc("POST /api/reviews/comments/{id}/feedback", cfg.FeedbackHandler.Submit)
	}
	if cfg.ExplainHandler != nil {
		api.HandleFunc("POST /api/reviews/comments/{id}/explain", cfg.ExplainHandler.Explain)
	}
	if cfg.AuditHandler != nil {
		adminFunc("GET /api/audit", cfg.AuditHandler.List)
		adminFunc("GET /api/audit/export", cfg.AuditHandler.Export)
	}
	if cfg.RetentionHandler != nil {
		adminFunc("GET /api/settings/retention", cfg.RetentionHandler.Get)
		adminFunc("PUT /api/settings/retention", cfg.RetentionHandler.Put)
		adminFunc("DELETE /api/users/{login}/data", cfg.RetentionHandler.EraseUser)
	}
	if cfg.SSOHandler != nil {
		adminFunc("GET /api/settings/sso", cfg.SSOHandler.GetConfig)
		adminFunc("PUT /api/settings/sso", cfg.SSOHandler.PutConfig)
		adminFunc("DELETE /api/settings/sso", cfg.SSOHandler.DeleteConfig)
	}
	if cfg.APITokenHandler != nil {
		// Token routes are accessible to all authenticated roles — every user can
		// manage their own tokens. The handlers enforce WHERE user_id = ? so users
		// can only see and revoke their own tokens.
		api.HandleFunc("GET /api/tokens", cfg.APITokenHandler.List)
		api.HandleFunc("POST /api/tokens", cfg.APITokenHandler.Create)
		api.HandleFunc("DELETE /api/tokens/{id}", cfg.APITokenHandler.Revoke)
	}
	if cfg.IntegrationHandler != nil {
		adminFunc("GET /api/settings/integrations/jira", cfg.IntegrationHandler.GetJira)
		adminFunc("PUT /api/settings/integrations/jira", cfg.IntegrationHandler.PutJira)
		adminFunc("DELETE /api/settings/integrations/jira", cfg.IntegrationHandler.DeleteJira)
		adminFunc("POST /api/settings/integrations/jira/test", cfg.IntegrationHandler.TestJira)
	}
	if cfg.SlackAppHandler != nil {
		adminFunc("GET /api/settings/slack-app", cfg.SlackAppHandler.Get)
		adminFunc("PUT /api/settings/slack-app", cfg.SlackAppHandler.Put)
		adminFunc("DELETE /api/settings/slack-app", cfg.SlackAppHandler.Delete)
		adminFunc("POST /api/settings/slack-app/test", cfg.SlackAppHandler.Test)
	}

	protected := middleware.Auth(cfg.JWTSecret, cfg.DB)(api)
	if cfg.RateLimiter != nil {
		// Rate limit after auth so we have user identity for keying.
		rateLimited := cfg.RateLimiter.Middleware(func(r *http.Request) string {
			user := middleware.UserFromCtx(r.Context())
			if user != nil {
				return fmt.Sprintf("user:%d", user.ID)
			}
			return r.RemoteAddr
		})
		mux.Handle("/api/", rateLimited(protected))
	} else {
		mux.Handle("/api/", protected)
	}

	return middleware.CORS(cfg.AllowedOrigin)(mux)
}

// NewSimpleRouter kept for backward compat with tests that pass a single handler.
func NewSimpleRouter(handler WebhookHandlerIface) http.Handler {
	cfg := RouterConfig{WebhookHandler: handler}
	return NewRouter(cfg)
}
