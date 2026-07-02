package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
)

type AuthUser struct {
	ID        uint
	Login     string
	Role      string
	SessionID string
}

type contextKey string

const userCtxKey contextKey = "auth_user"

func UserFromCtx(ctx context.Context) *AuthUser {
	u, _ := ctx.Value(userCtxKey).(*AuthUser)
	return u
}

// Auth validates the Bearer JWT and, when db is non-nil, verifies the session is active.
func Auth(secret string, db *gorm.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(header, "Bearer ")

			// API token auth: tokens start with "prt_".
			if strings.HasPrefix(tokenStr, "prt_") && db != nil {
				sum := sha256.Sum256([]byte(tokenStr))
				hash := hex.EncodeToString(sum[:])
				var apiToken models.APIToken
				if err := db.Where("hash = ?", hash).First(&apiToken).Error; err == nil {
					// Token found — check expiry.
					if apiToken.ExpiresAt != nil && time.Now().After(*apiToken.ExpiresAt) {
						http.Error(w, `{"error":"token expired"}`, http.StatusUnauthorized)
						return
					}
					// Enforce token scope: read-only tokens may only call safe (GET/HEAD)
					// methods, matching the promise shown in the UI.
					if apiToken.Scope == "read" && r.Method != http.MethodGet && r.Method != http.MethodHead {
						http.Error(w, `{"error":"read-only token cannot perform write operations"}`, http.StatusForbidden)
						return
					}
					// Fetch the user and verify they're still active.
					var u models.User
					if db.First(&u, apiToken.UserID).Error == nil {
						if u.Status != "active" {
							http.Error(w, `{"error":"account suspended"}`, http.StatusForbidden)
							return
						}
						// Update last_used_at asynchronously.
						now := time.Now()
						go db.Model(&apiToken).Update("last_used_at", now)

						ctx := context.WithValue(r.Context(), userCtxKey, &AuthUser{
							ID:        u.ID,
							Login:     u.Login,
							Role:      u.Role,
							SessionID: "",
						})
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			// Pre-auth tokens (issued before the consent screen) are not valid API
			// credentials — they exist only to complete login at /auth/github/continue.
			if typ, _ := claims["typ"].(string); typ == "preauth" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			sub, _ := claims["sub"].(float64)
			login, _ := claims["login"].(string)
			role, _ := claims["role"].(string)
			sid, _ := claims["sid"].(string)

			// Session validation — skip if no DB or no session ID (legacy tokens).
			if db != nil && sid != "" {
				var s models.Session
				if err := db.Where("id = ? AND expires_at > ?", sid, time.Now()).First(&s).Error; err != nil {
					http.Error(w, `{"error":"session expired or revoked"}`, http.StatusUnauthorized)
					return
				}
			}

			ctx := context.WithValue(r.Context(), userCtxKey, &AuthUser{
				ID:        uint(sub),
				Login:     login,
				Role:      role,
				SessionID: sid,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole rejects requests where the authenticated user's role is not in the allowed list.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromCtx(r.Context())
			if user == nil || !allowed[user.Role] {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
