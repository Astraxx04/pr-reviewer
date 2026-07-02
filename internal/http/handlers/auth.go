package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	gogithub "github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"
	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/audit"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"github.com/Astraxx04/pr-reviewer/internal/db/repo"
	"github.com/Astraxx04/pr-reviewer/internal/http/middleware"
	"github.com/Astraxx04/pr-reviewer/internal/notifications"
	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

type AuthHandler struct {
	oauth2Cfg     *oauth2.Config
	jwtSecret     []byte
	jwtTTL        time.Duration
	frontendURL   string
	userRepo      *repo.UserRepo
	sessionRepo   *repo.SessionRepo
	db            *gorm.DB
	requiredOrg   string
	encryptionKey string
}

func NewAuthHandler(
	clientID, clientSecret, jwtSecret, frontendURL, serverURL string,
	db *gorm.DB,
	requiredOrg string,
	jwtTTLHours int,
	encryptionKey string,
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
		jwtSecret:     []byte(jwtSecret),
		jwtTTL:        ttl,
		frontendURL:   frontendURL,
		userRepo:      repo.NewUserRepo(db),
		sessionRepo:   repo.NewSessionRepo(db),
		db:            db,
		requiredOrg:   requiredOrg,
		encryptionKey: encryptionKey,
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

	// CLI loopback login: remember where to hand the token back, but only if it's a
	// safe localhost address. This prevents a crafted cli_redirect from exfiltrating
	// a token to an attacker-controlled host.
	if cliRedirect := r.URL.Query().Get("cli_redirect"); cliRedirect != "" && isLoopbackURL(cliRedirect) {
		http.SetCookie(w, &http.Cookie{
			Name:     "cli_redirect",
			Value:    cliRedirect,
			Path:     "/",
			MaxAge:   300,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	// Carry an invite token through the OAuth round-trip in a short-lived cookie.
	if inviteToken := r.URL.Query().Get("invite"); inviteToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_invite",
			Value:    inviteToken,
			Path:     "/",
			MaxAge:   300,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	http.Redirect(w, r, h.oauth2Cfg.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

// isLoopbackURL reports whether raw is an http(s) URL pointing at the local
// machine. Only such URLs are accepted as CLI token-return targets.
func isLoopbackURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	switch u.Hostname() {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
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

	// Org membership check — always enforced; REQUIRED_ORG is a mandatory env var.
	// Use GetOrgMembership (/user/memberships/orgs/{org}) rather than IsMember
	// (/orgs/{org}/members/{username}): the latter returns 302 for private memberships,
	// which go-github parses as false/nil (no error), silently blocking valid members.
	memberStart := time.Now()
	membership, _, memberErr := ghClient.Organizations.GetOrgMembership(r.Context(), "", h.requiredOrg)
	logger.ExternalCall(r.Context(), "github", "Organizations.GetOrgMembership", memberStart, memberErr, "org", h.requiredOrg, "login", ghUser.GetLogin())
	if memberErr != nil || membership.GetState() != "active" {
		http.Redirect(w, r,
			fmt.Sprintf("%s/auth/error?reason=org_required&org=%s", h.frontendURL, url.QueryEscape(h.requiredOrg)),
			http.StatusTemporaryRedirect,
		)
		return
	}

	// Fetch verified email list once — used for both primary-email resolution and
	// invite email matching (avoids a second round-trip on the invite path).
	emailListStart := time.Now()
	ghEmails, _, _ := ghClient.Users.ListEmails(r.Context(), &gogithub.ListOptions{PerPage: 100})
	logger.ExternalCall(r.Context(), "github", "Users.ListEmails", emailListStart, nil)

	// Read invite token from the short-lived cookie set by Login.
	inviteCookie, _ := r.Cookie("oauth_invite")
	http.SetCookie(w, &http.Cookie{Name: "oauth_invite", MaxAge: -1, Path: "/"})

	var acceptedInvite *models.Invite
	if inviteCookie != nil && inviteCookie.Value != "" {
		sum := sha256.Sum256([]byte(inviteCookie.Value))
		tokenHash := hex.EncodeToString(sum[:])

		var invite models.Invite
		if err := h.db.WithContext(r.Context()).
			Where("token_hash = ? AND accepted_at IS NULL AND expires_at > ?", tokenHash, time.Now()).
			First(&invite).Error; err != nil {
			http.Redirect(w, r, h.frontendURL+"/auth/error?reason=invite_invalid", http.StatusTemporaryRedirect)
			return
		}

		if !emailVerifiedOnGitHub(ghEmails, invite.Email) {
			http.Redirect(w, r,
				fmt.Sprintf("%s/auth/error?reason=email_mismatch&email=%s&login=%s",
					h.frontendURL,
					url.QueryEscape(invite.Email),
					url.QueryEscape(ghUser.GetLogin()),
				),
				http.StatusTemporaryRedirect,
			)
			return
		}
		acceptedInvite = &invite
	}

	// Determine role/status for this login attempt.
	userCount, _ := h.userRepo.Count(r.Context())

	// Resolve the notification email from the already-fetched list; fall back to
	// any address already on file to avoid blanking it on re-login.
	email := resolvePrimaryEmailFromList(ghUser.GetEmail(), ghEmails)
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

	if userCount == 0 {
		// First user becomes owner (initial setup, no invite required).
		newUser.Role = "owner"
		newUser.Status = "active"
	} else if acceptedInvite != nil {
		// New user arriving via a valid invite link.
		newUser.Role = acceptedInvite.Role
		newUser.Status = "active"
	} else {
		// Returning user — preserve their current role/status.
		if existing, err := h.userRepo.FindByGithubID(r.Context(), ghUser.GetID()); err == nil {
			newUser.Role = existing.Role
			newUser.Status = existing.Status
		} else {
			// Unknown user with no invite token.
			http.Redirect(w, r, h.frontendURL+"/auth/error?reason=invite_required", http.StatusTemporaryRedirect)
			return
		}
	}

	if err := h.userRepo.Upsert(r.Context(), newUser); err != nil {
		writeError(w, http.StatusInternalServerError, "user upsert failed")
		return
	}

	// Re-fetch to get the actual DB-assigned role/status.
	dbUser, err := h.userRepo.FindByGithubID(r.Context(), newUser.GithubID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}

	// Block suspended users before any session is created.
	if dbUser.Status == "suspended" {
		http.Redirect(w, r, h.frontendURL+"/auth/error?reason=suspended", http.StatusTemporaryRedirect)
		return
	}

	// Mark invite accepted and notify the inviting admin.
	if acceptedInvite != nil {
		now := time.Now()
		h.db.WithContext(r.Context()).Model(acceptedInvite).Updates(map[string]any{
			"accepted_at": &now,
			"accepted_by": ghUser.GetLogin(),
		})
		audit.Log(h.db, r, ghUser.GetLogin(), dbUser.ID, "invite.accepted", "user", acceptedInvite.ID,
			nil, map[string]any{"email": acceptedInvite.Email, "role": acceptedInvite.Role})
		go h.sendAcceptanceNotification(context.Background(), acceptedInvite, ghUser.GetLogin())
	}

	// Kick off a background repo-access sync for this user so fail-closed repo
	// visibility is populated before their first page load.
	go syncLoginRepoAccess(context.Background(), h.db, h.encryptionKey, ghUser.GetLogin())

	// Detect a CLI login from the loopback cookie and carry the target forward in
	// the pre-auth token, so we don't depend on the cookie surviving the consent
	// click. No session or usable token is created until the user confirms.
	cliRedirect := ""
	if c, err := r.Cookie("cli_redirect"); err == nil && c.Value != "" && isLoopbackURL(c.Value) {
		cliRedirect = c.Value
		http.SetCookie(w, &http.Cookie{Name: "cli_redirect", MaxAge: -1, Path: "/"})
	}

	// Mint a short-lived pre-auth token. It is NOT a usable API credential (the auth
	// middleware rejects typ=preauth) — it only lets the consent screen complete the
	// login via POST /auth/github/continue.
	preAuth := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": float64(dbUser.ID),
		"cli": cliRedirect,
		"typ": "preauth",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})
	signedPre, err := preAuth.SignedString(h.jwtSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token sign failed")
		return
	}

	// Redirect off this single-use OAuth callback to a reloadable consent URL — so a
	// page refresh re-renders the consent screen instead of replaying a consumed
	// code (which would fail with "invalid state"). CLI logins go to the backend's
	// self-contained consent page (no frontend dependency); web logins go to the
	// frontend consent route so it matches the dashboard. The session and real token
	// are created only when the user confirms via POST /auth/github/continue.
	if cliRedirect != "" {
		http.Redirect(w, r, "/auth/github/consent?t="+url.QueryEscape(signedPre), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r,
		fmt.Sprintf("%s/auth/consent?t=%s&u=%s", h.frontendURL, url.QueryEscape(signedPre), url.QueryEscape(dbUser.Login)),
		http.StatusSeeOther,
	)
}

// ConsentPage renders the backend's self-contained consent screen for a CLI login.
// It is reachable by GET (so a browser refresh is safe) and only re-renders the
// form from the still-valid pre-auth token; nothing is mutated here.
func (h *AuthHandler) ConsentPage(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("t")
	userID, _, ok := h.parsePreAuth(tokenStr)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid or expired login request")
		return
	}
	var dbUser models.User
	if err := h.db.WithContext(r.Context()).First(&dbUser, userID).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}
	renderConsentPage(w, dbUser.Login, tokenStr)
}

// parsePreAuth validates a pre-auth token and returns the user ID and CLI redirect
// it carries. ok is false if the token is missing, malformed, wrong type, or expired.
func (h *AuthHandler) parsePreAuth(tokenStr string) (userID uint, cliRedirect string, ok bool) {
	claims := jwt.MapClaims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return h.jwtSecret, nil
	})
	if err != nil || !tok.Valid {
		return 0, "", false
	}
	if typ, _ := claims["typ"].(string); typ != "preauth" {
		return 0, "", false
	}
	subF, _ := claims["sub"].(float64)
	if uint(subF) == 0 {
		return 0, "", false
	}
	cli, _ := claims["cli"].(string)
	return uint(subF), cli, true
}

// ContinueLogin completes a login after the user confirms on the consent screen.
// It validates the short-lived pre-auth token and only then creates the session
// and issues the real token — so abandoned consent screens leave no session rows.
func (h *AuthHandler) ContinueLogin(w http.ResponseWriter, r *http.Request) {
	userID, cliRedirect, ok := h.parsePreAuth(r.FormValue("t"))
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid or expired login request")
		return
	}

	// Re-load the user so role/status reflect the current DB, not the token.
	var dbUser models.User
	if err := h.db.WithContext(r.Context()).First(&dbUser, userID).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}
	if dbUser.Status == "suspended" {
		http.Redirect(w, r, h.frontendURL+"/auth/error?reason=suspended", http.StatusSeeOther)
		return
	}

	// CLI logins (loopback target) get a longer-lived token; web uses the standard TTL.
	ttl := h.jwtTTL
	isCLI := cliRedirect != "" && isLoopbackURL(cliRedirect)
	if isCLI {
		ttl = cliTokenTTL
	}

	sessionID := randomHex(16)
	expiresAt := time.Now().Add(ttl)
	_ = h.sessionRepo.Create(r.Context(), &models.Session{
		ID:           sessionID,
		UserID:       dbUser.ID,
		UserAgent:    r.Header.Get("User-Agent"),
		IPAddress:    r.RemoteAddr,
		LastActiveAt: time.Now(),
		ExpiresAt:    expiresAt,
		CreatedAt:    time.Now(),
	})

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

	if isCLI {
		sep := "?"
		if strings.Contains(cliRedirect, "?") {
			sep = "&"
		}
		http.Redirect(w, r, cliRedirect+sep+"token="+url.QueryEscape(signed), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, h.frontendURL+"/auth/callback?token="+signed, http.StatusSeeOther)
}

// cliTokenTTL is how long a token minted through the CLI browser flow stays valid.
const cliTokenTTL = 7 * 24 * time.Hour

// consentTmpl is the app's own "authorize this login" screen, shown after GitHub
// auth and before any session/token is created. The submit POSTs the pre-auth
// token to /auth/github/continue; a POST (not a link) ensures the approval is a
// deliberate action that browsers won't prefetch.
var consentTmpl = template.Must(template.New("consent").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>Sign in · PR Reviewer</title>
<style>
 body{font-family:system-ui,-apple-system,sans-serif;background:#0d1117;color:#e6edf3;display:flex;min-height:100vh;margin:0;align-items:center;justify-content:center}
 .card{background:#161b22;border:1px solid #30363d;border-radius:12px;padding:2.5rem;max-width:380px;text-align:center}
 h1{font-size:1.25rem;margin:0 0 .5rem}
 p{color:#9da7b3;margin:.25rem 0 1.5rem;font-size:.95rem}
 .who{color:#e6edf3;font-weight:600}
 button.btn{width:100%;border:0;cursor:pointer;background:#238636;color:#fff;padding:.7rem 1rem;border-radius:8px;font-weight:600;font-size:1rem}
 button.btn:hover{background:#2ea043}
 .ctx{margin-top:1rem;font-size:.8rem;color:#6e7681}
</style></head>
<body><div class="card">
 <h1>Authorize sign-in</h1>
 <p>You're signing in to PR Reviewer as <span class="who">{{.Login}}</span>.</p>
 <form method="POST" action="/auth/github/continue">
  <input type="hidden" name="t" value="{{.PreAuth}}">
  <button class="btn" type="submit">Yes, continue</button>
 </form>
 <div class="ctx">This signs in the command-line tool on this machine.</div>
</div></body></html>`))

// renderConsentPage writes the backend's self-contained confirmation screen, used
// for CLI logins (which must work without the frontend running).
func renderConsentPage(w http.ResponseWriter, login, preAuth string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = consentTmpl.Execute(w, struct {
		Login   string
		PreAuth string
	}{Login: login, PreAuth: preAuth})
}

// resolvePrimaryEmailFromList picks the best notification email from a pre-fetched list.
// Returns the profile email if non-empty, then the primary verified address, then any
// verified address. Returns "" if none can be resolved.
func resolvePrimaryEmailFromList(profileEmail string, emails []*gogithub.UserEmail) string {
	if profileEmail != "" {
		return profileEmail
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

// emailVerifiedOnGitHub reports whether target appears as a verified email on the
// GitHub account described by emails. Comparison is case-insensitive.
func emailVerifiedOnGitHub(emails []*gogithub.UserEmail, target string) bool {
	for _, e := range emails {
		if e.GetVerified() && strings.EqualFold(e.GetEmail(), target) {
			return true
		}
	}
	return false
}

// sendAcceptanceNotification emails the inviting admin to say their invitee has joined.
// Looks up the admin's email from the users table and sends via the installation's
// configured email channel. Runs as a fire-and-forget goroutine — errors are silently dropped.
func (h *AuthHandler) sendAcceptanceNotification(ctx context.Context, invite *models.Invite, acceptorLogin string) {
	var admin models.User
	if h.db.WithContext(ctx).Where("login = ?", invite.InvitedBy).First(&admin).Error != nil || admin.Email == "" {
		return
	}

	var cfgs []models.NotificationConfig
	h.db.WithContext(ctx).Where("channel = ? AND enabled = true", "email").Find(&cfgs)
	if len(cfgs) == 0 {
		return
	}

	subject := fmt.Sprintf("%s accepted your invitation", acceptorLogin)
	body := fmt.Sprintf(`<p><strong>%s</strong> accepted your invitation and joined as <strong>%s</strong>.</p>
<table style="border-collapse:collapse">
  <tr><td style="padding:4px 12px 4px 0">GitHub account:</td><td>%s</td></tr>
  <tr><td style="padding:4px 12px 4px 0">Email used:</td><td>%s</td></tr>
  <tr><td style="padding:4px 12px 4px 0">Accepted at:</td><td>%s UTC</td></tr>
</table>
<p><a href="%s/settings/team">View team →</a></p>`,
		acceptorLogin, invite.Role, acceptorLogin, invite.Email,
		time.Now().UTC().Format("2006-01-02 15:04"), h.frontendURL)

	for _, cfg := range cfgs {
		var ec notifications.EmailChannelConfig
		if err := json.Unmarshal(cfg.Config, &ec); err != nil {
			continue
		}
		smtpCfg, from := notifications.ResolveEmail(ec)
		_ = notifications.SendEmail(ctx, smtpCfg, from, []string{admin.Email}, subject, body)
	}
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
