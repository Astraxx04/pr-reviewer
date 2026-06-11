package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	gogithub "github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"
	"gorm.io/gorm"

	"pr-reviewer/internal/db/models"
	"pr-reviewer/internal/db/repo"
	"pr-reviewer/internal/http/middleware"
	"pr-reviewer/pkg/logger"
)

type AuthHandler struct {
	oauth2Cfg   *oauth2.Config
	jwtSecret   []byte
	jwtTTL      time.Duration
	frontendURL string
	userRepo    *repo.UserRepo
	sessionRepo *repo.SessionRepo
	db          *gorm.DB
	requiredOrg string
	inviteOnly  bool
}

func NewAuthHandler(
	clientID, clientSecret, jwtSecret, frontendURL, serverURL string,
	db *gorm.DB,
	requiredOrg string,
	inviteOnly bool,
	jwtTTLHours int,
) *AuthHandler {
	ttl := time.Duration(jwtTTLHours) * time.Hour
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &AuthHandler{
		oauth2Cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       []string{"read:user", "user:email", "read:org"},
			Endpoint:     githuboauth.Endpoint,
			RedirectURL:  serverURL + "/auth/github/callback",
		},
		jwtSecret:   []byte(jwtSecret),
		jwtTTL:      ttl,
		frontendURL: frontendURL,
		userRepo:    repo.NewUserRepo(db),
		sessionRepo: repo.NewSessionRepo(db),
		db:          db,
		requiredOrg: requiredOrg,
		inviteOnly:  inviteOnly,
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	state := randomHex(16)
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.oauth2Cfg.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("oauth_state")
	if err != nil || cookie.Value != r.URL.Query().Get("state") {
		writeError(w, http.StatusBadRequest, "invalid state")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", MaxAge: -1, Path: "/"})

	exchangeStart := time.Now()
	token, err := h.oauth2Cfg.Exchange(r.Context(), r.URL.Query().Get("code"))
	logger.ExternalCall(r.Context(), "github-oauth", "Exchange", exchangeStart, err)
	if err != nil {
		writeError(w, http.StatusBadRequest, "code exchange failed")
		return
	}

	ghClient := gogithub.NewClient(h.oauth2Cfg.Client(context.Background(), token))
	userStart := time.Now()
	ghUser, _, err := ghClient.Users.Get(r.Context(), "")
	logger.ExternalCall(r.Context(), "github", "Users.Get", userStart, err)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch github user")
		return
	}

	// 3.1 — Org membership gate.
	if h.requiredOrg != "" {
		memberStart := time.Now()
		isMember, _, err := ghClient.Organizations.IsMember(r.Context(), h.requiredOrg, ghUser.GetLogin())
		logger.ExternalCall(r.Context(), "github", "Organizations.IsMember", memberStart, err, "org", h.requiredOrg, "login", ghUser.GetLogin())
		if err != nil || !isMember {
			http.Redirect(w, r,
				fmt.Sprintf("%s/auth/error?reason=org_required&org=%s", h.frontendURL, h.requiredOrg),
				http.StatusTemporaryRedirect,
			)
			return
		}
	}

	// Determine role/status for this login attempt.
	userCount, _ := h.userRepo.Count(r.Context())

	// Resolve the notification email. If GitHub gives us nothing this time, keep any
	// address we already have on file rather than blanking it on the upsert.
	email := resolvePrimaryEmail(r.Context(), ghClient, ghUser.GetEmail())
	if email == "" {
		if existing, err := h.userRepo.FindByGithubID(r.Context(), ghUser.GetID()); err == nil {
			email = existing.Email
		}
	}

	newUser := &models.User{
		GithubID:  ghUser.GetID(),
		Login:     ghUser.GetLogin(),
		Email:     email,
		AvatarURL: ghUser.GetAvatarURL(),
	}

	// 3.2 — First user becomes owner; check TeamMember pre-authorization; else invite-only or viewer.
	if userCount == 0 {
		newUser.Role = "owner"
		newUser.Status = "active"
	} else {
		// Check if an admin pre-authorized this login with a specific role.
		var tm models.TeamMember
		if h.db.Where("login = ?", ghUser.GetLogin()).First(&tm).Error == nil {
			newUser.Role = tm.Role
			newUser.Status = "active"
		} else if h.inviteOnly {
			// Only set pending for genuinely new users; existing users keep their status.
			newUser.Status = "pending"
			newUser.Role = "viewer"
		} else {
			newUser.Role = "viewer"
			newUser.Status = "active"
		}
	}

	if err := h.userRepo.Upsert(r.Context(), newUser); err != nil {
		writeError(w, http.StatusInternalServerError, "user upsert failed")
		return
	}

	// Re-fetch to get the actual DB-assigned role/status (preserves existing users' roles).
	dbUser, err := h.userRepo.FindByGithubID(r.Context(), newUser.GithubID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}

	// Reject pending users with a redirect.
	if dbUser.Status == "pending" {
		http.Redirect(w, r,
			h.frontendURL+"/auth/error?reason=pending_approval",
			http.StatusTemporaryRedirect,
		)
		return
	}

	// 3.5 — Create a session record.
	sessionID := randomHex(16)
	expiresAt := time.Now().Add(h.jwtTTL)
	_ = h.sessionRepo.Create(r.Context(), &models.Session{
		ID:           sessionID,
		UserID:       dbUser.ID,
		UserAgent:    r.Header.Get("User-Agent"),
		IPAddress:    r.RemoteAddr,
		LastActiveAt: time.Now(),
		ExpiresAt:    expiresAt,
		CreatedAt:    time.Now(),
	})

	// 3.3 — Embed role and session ID in JWT.
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   float64(dbUser.ID),
		"login": dbUser.Login,
		"role":  dbUser.Role,
		"sid":   sessionID,
		"exp":   expiresAt.Unix(),
	})
	signed, err := jwtToken.SignedString(h.jwtSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token sign failed")
		return
	}

	http.Redirect(w, r, h.frontendURL+"/auth/callback?token="+signed, http.StatusTemporaryRedirect)
}

// resolvePrimaryEmail returns the user's email for notification routing. GitHub's
// profile email (ghUser.GetEmail) is blank when the user keeps it private — the
// default — so when it's empty we fall back to GET /user/emails (the OAuth flow
// already requests the user:email scope) and pick the primary verified address,
// then any verified address. Returns "" if none can be resolved.
func resolvePrimaryEmail(ctx context.Context, ghClient *gogithub.Client, profileEmail string) string {
	if profileEmail != "" {
		return profileEmail
	}
	start := time.Now()
	emails, _, err := ghClient.Users.ListEmails(ctx, &gogithub.ListOptions{PerPage: 100})
	logger.ExternalCall(ctx, "github", "Users.ListEmails", start, err)
	if err != nil {
		return ""
	}
	var firstVerified string
	for _, e := range emails {
		if !e.GetVerified() {
			continue
		}
		if e.GetPrimary() {
			return e.GetEmail()
		}
		if firstVerified == "" {
			firstVerified = e.GetEmail()
		}
	}
	return firstVerified
}

// Me returns the current user's profile and role.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromCtx(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	dbUser, err := h.userRepo.FindByID(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         dbUser.ID,
		"login":      dbUser.Login,
		"email":      dbUser.Email,
		"avatar_url": dbUser.AvatarURL,
		"role":       dbUser.Role,
		"status":     dbUser.Status,
	})
}

// Logout revokes the current session.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromCtx(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if user.SessionID != "" {
		_ = h.sessionRepo.Delete(r.Context(), user.SessionID, user.ID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func isAdmin(user *middleware.AuthUser) bool {
	return user != nil && (user.Role == "owner" || user.Role == "admin")
}

// decodeJSON is a helper for decoding request bodies.
func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
