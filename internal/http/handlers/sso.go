package handlers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	dbpkg "github.com/Astraxx04/pr-reviewer/internal/db"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

type SSOHandler struct {
	db            *gorm.DB
	encryptionKey string
	serverURL     string
	frontendURL   string
	jwtSecret     string
	jwtTTLHours   int

	mu           sync.RWMutex
	cachedConfig *oidcDiscovery
	cachedJWKS   map[string]*rsa.PublicKey
	cacheExpiry  time.Time
}

type oidcDiscovery struct {
	Issuer   string `json:"issuer"`
	AuthURL  string `json:"authorization_endpoint"`
	TokenURL string `json:"token_endpoint"`
	JWKSURI  string `json:"jwks_uri"`
}

func NewSSOHandler(db *gorm.DB, encryptionKey, serverURL, frontendURL, jwtSecret string, jwtTTLHours int) *SSOHandler {
	if jwtTTLHours == 0 {
		jwtTTLHours = 24
	}
	return &SSOHandler{
		db:            db,
		encryptionKey: encryptionKey,
		serverURL:     strings.TrimRight(serverURL, "/"),
		frontendURL:   strings.TrimRight(frontendURL, "/"),
		jwtSecret:     jwtSecret,
		jwtTTLHours:   jwtTTLHours,
	}
}

// GetConfig returns the current OIDC configuration (admin only).
func (h *SSOHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	var cfg models.OIDCConfig
	if err := h.db.WithContext(r.Context()).First(&cfg).Error; err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"configured":        true,
		"issuer":            cfg.Issuer,
		"client_id":         cfg.ClientID,
		"has_client_secret": cfg.ClientSecretEncrypted != "",
		"redirect_url":      cfg.RedirectURL,
		"attribute_mapping": cfg.AttributeMapping,
		"role_mapping":      cfg.RoleMapping,
		"enforced":          cfg.Enforced,
		"enabled":           cfg.Enabled,
	})
}

// PutConfig saves or updates the OIDC SSO configuration (admin only).
func (h *SSOHandler) PutConfig(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	var body struct {
		Issuer           string         `json:"issuer"`
		ClientID         string         `json:"client_id"`
		ClientSecret     string         `json:"client_secret"` // raw; encrypted before store
		RedirectURL      string         `json:"redirect_url"`
		AttributeMapping map[string]any `json:"attribute_mapping"`
		RoleMapping      map[string]any `json:"role_mapping"`
		Enforced         bool           `json:"enforced"`
		Enabled          bool           `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.Issuer == "" || body.ClientID == "" {
		writeError(w, http.StatusBadRequest, "issuer and client_id required")
		return
	}

	var secretEnc string
	if body.ClientSecret != "" {
		enc, err := dbpkg.Encrypt(body.ClientSecret, h.encryptionKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt secret")
			return
		}
		secretEnc = enc
	} else {
		// Preserve existing secret.
		var existing models.OIDCConfig
		h.db.WithContext(r.Context()).First(&existing)
		secretEnc = existing.ClientSecretEncrypted
	}

	attrJSON, _ := json.Marshal(body.AttributeMapping)
	roleJSON, _ := json.Marshal(body.RoleMapping)

	cfg := models.OIDCConfig{
		ID:                    1, // single-row config
		Issuer:                body.Issuer,
		ClientID:              body.ClientID,
		ClientSecretEncrypted: secretEnc,
		RedirectURL:           body.RedirectURL,
		AttributeMapping:      attrJSON,
		RoleMapping:           roleJSON,
		Enforced:              body.Enforced,
		Enabled:               body.Enabled,
	}
	h.db.WithContext(r.Context()).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"issuer", "client_id", "client_secret_encrypted",
			"redirect_url", "attribute_mapping", "role_mapping",
			"enforced", "enabled", "updated_at",
		}),
	}).Create(&cfg)

	// Invalidate cached discovery doc.
	h.mu.Lock()
	h.cachedConfig = nil
	h.cachedJWKS = nil
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// DeleteConfig removes the OIDC configuration (admin only).
func (h *SSOHandler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if !isAdmin(user) {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	h.db.WithContext(r.Context()).Where("id = 1").Delete(&models.OIDCConfig{})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Login initiates the OIDC authorization code flow.
func (h *SSOHandler) Login(w http.ResponseWriter, r *http.Request) {
	cfg, secret, disc, err := h.loadConfig(r.Context())
	if err != nil || !cfg.Enabled {
		http.Error(w, "SSO not configured", http.StatusServiceUnavailable)
		return
	}

	redirectURL := cfg.RedirectURL
	if redirectURL == "" {
		redirectURL = h.serverURL + "/auth/oidc/callback"
	}

	oa2Cfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: secret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  disc.AuthURL,
			TokenURL: disc.TokenURL,
		},
		RedirectURL: redirectURL,
		Scopes:      []string{"openid", "email", "profile"},
	}

	state := uuid.New().String()
	// Store state in a short-lived cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, oa2Cfg.AuthCodeURL(state), http.StatusFound)
}

// Callback handles the OIDC redirect back from the IdP.
func (h *SSOHandler) Callback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oidc_state")
	if err != nil || r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "oidc_state", MaxAge: -1, Path: "/"})

	cfg, secret, disc, err := h.loadConfig(r.Context())
	if err != nil {
		http.Error(w, "SSO not configured", http.StatusServiceUnavailable)
		return
	}

	redirectURL := cfg.RedirectURL
	if redirectURL == "" {
		redirectURL = h.serverURL + "/auth/oidc/callback"
	}

	oa2Cfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: secret,
		Endpoint:     oauth2.Endpoint{AuthURL: disc.AuthURL, TokenURL: disc.TokenURL},
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
	}

	code := r.URL.Query().Get("code")
	tok, err := oa2Cfg.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	idTokenRaw, ok := tok.Extra("id_token").(string)
	if !ok {
		http.Error(w, "missing id_token", http.StatusBadRequest)
		return
	}

	// Parse and validate the ID token.
	jwks, err := h.getJWKS(disc.JWKSURI)
	if err != nil {
		http.Error(w, "failed to fetch JWKS: "+err.Error(), http.StatusInternalServerError)
		return
	}

	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(idTokenRaw, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		key, ok := jwks[kid]
		if !ok {
			// Try empty key (some IdPs don't set kid).
			for _, k := range jwks {
				return k, nil
			}
			return nil, fmt.Errorf("no matching key for kid=%s", kid)
		}
		return key, nil
	}, jwt.WithIssuedAt(), jwt.WithAudience(cfg.ClientID))
	if err != nil {
		http.Error(w, "invalid id_token: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Extract user info from claims.
	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	if sub == "" {
		http.Error(w, "missing sub claim", http.StatusBadRequest)
		return
	}

	// Upsert user — use email as login if no preferred_username.
	login := email
	if pu, ok := claims["preferred_username"].(string); ok && pu != "" {
		login = pu
	}
	if login == "" {
		login = sub
	}

	// Determine role from role mapping.
	role := h.mapRole(cfg, claims)

	dbUser := &models.User{
		GithubID: 0, // OIDC users have GithubID=0
		Login:    login,
		Email:    email,
		Role:     role,
		Status:   "active",
	}
	// Try to find existing by login.
	var existing models.User
	if h.db.Where("login = ?", login).First(&existing).Error == nil {
		dbUser.ID = existing.ID
		dbUser.GithubID = existing.GithubID
		h.db.WithContext(r.Context()).Save(dbUser)
	} else {
		h.db.WithContext(r.Context()).Create(dbUser)
	}
	_ = name // used for display; stored in email field for now

	// Create session and issue JWT.
	sid := uuid.New().String()
	expiry := time.Now().Add(time.Duration(h.jwtTTLHours) * time.Hour)
	h.db.WithContext(r.Context()).Create(&models.Session{
		ID:           sid,
		UserID:       dbUser.ID,
		UserAgent:    r.UserAgent(),
		IPAddress:    r.RemoteAddr,
		LastActiveAt: time.Now(),
		ExpiresAt:    expiry,
	})

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   float64(dbUser.ID),
		"login": dbUser.Login,
		"role":  dbUser.Role,
		"sid":   sid,
		"exp":   expiry.Unix(),
	})
	signed, err := jwtToken.SignedString([]byte(h.jwtSecret))
	if err != nil {
		http.Error(w, "failed to sign token", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, h.frontendURL+"/auth/callback?token="+signed, http.StatusFound)
}

// mapRole derives a platform role from OIDC claims using the role_mapping config.
func (h *SSOHandler) mapRole(cfg *models.OIDCConfig, claims jwt.MapClaims) string {
	if len(cfg.RoleMapping) == 0 {
		return "viewer"
	}
	var mapping map[string][]string
	if err := json.Unmarshal(cfg.RoleMapping, &mapping); err != nil {
		return "viewer"
	}

	var attrMapping map[string]string
	_ = json.Unmarshal(cfg.AttributeMapping, &attrMapping)

	roleClaimKey := "groups"
	if attrMapping != nil {
		if k, ok := attrMapping["role_claim"]; ok {
			roleClaimKey = k
		}
	}

	// Get the user's groups/roles from the claim.
	var userGroups []string
	switch v := claims[roleClaimKey].(type) {
	case []interface{}:
		for _, g := range v {
			if s, ok := g.(string); ok {
				userGroups = append(userGroups, s)
			}
		}
	case string:
		userGroups = []string{v}
	}

	// Check roles in priority order.
	for _, role := range []string{"owner", "admin", "reviewer", "viewer"} {
		groups, ok := mapping[role]
		if !ok {
			continue
		}
		for _, g := range groups {
			if g == "*" {
				return role
			}
			for _, ug := range userGroups {
				if g == ug {
					return role
				}
			}
		}
	}
	return "viewer"
}

// loadConfig fetches the OIDCConfig and decrypts the client secret.
func (h *SSOHandler) loadConfig(ctx context.Context) (*models.OIDCConfig, string, *oidcDiscovery, error) {
	var cfg models.OIDCConfig
	if err := h.db.WithContext(ctx).First(&cfg).Error; err != nil {
		return nil, "", nil, fmt.Errorf("SSO not configured")
	}
	secret, err := dbpkg.Decrypt(cfg.ClientSecretEncrypted, h.encryptionKey)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to decrypt client secret")
	}
	disc, err := h.fetchDiscovery(cfg.Issuer)
	if err != nil {
		return nil, "", nil, err
	}
	return &cfg, secret, disc, nil
}

func (h *SSOHandler) fetchDiscovery(issuer string) (*oidcDiscovery, error) {
	h.mu.RLock()
	if h.cachedConfig != nil && time.Now().Before(h.cacheExpiry) {
		d := h.cachedConfig
		h.mu.RUnlock()
		return d, nil
	}
	h.mu.RUnlock()

	url := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	start := time.Now()
	resp, err := http.Get(url) //nolint:noctx
	logger.ExternalCall(context.Background(), "oidc", "GET /.well-known/openid-configuration", start, err, "issuer", issuer)
	if err != nil {
		return nil, fmt.Errorf("discovery fetch failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var disc oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&disc); err != nil {
		return nil, fmt.Errorf("discovery parse failed: %w", err)
	}

	h.mu.Lock()
	h.cachedConfig = &disc
	h.cacheExpiry = time.Now().Add(1 * time.Hour)
	h.mu.Unlock()
	return &disc, nil
}

func (h *SSOHandler) getJWKS(uri string) (map[string]*rsa.PublicKey, error) {
	h.mu.RLock()
	if h.cachedJWKS != nil && time.Now().Before(h.cacheExpiry) {
		keys := h.cachedJWKS
		h.mu.RUnlock()
		return keys, nil
	}
	h.mu.RUnlock()

	start := time.Now()
	resp, err := http.Get(uri) //nolint:noctx
	logger.ExternalCall(context.Background(), "oidc", "GET JWKS", start, err, "uri", uri)
	if err != nil {
		return nil, fmt.Errorf("JWKS fetch failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, err
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		n := new(big.Int).SetBytes(nBytes)
		e := new(big.Int).SetBytes(eBytes)
		keys[k.Kid] = &rsa.PublicKey{N: n, E: int(e.Int64())}
	}

	h.mu.Lock()
	h.cachedJWKS = keys
	h.mu.Unlock()
	return keys, nil
}

// ensure rand is used (avoids import errors if not used elsewhere)
var _ = rand.Read
