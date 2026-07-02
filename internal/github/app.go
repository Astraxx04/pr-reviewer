package github

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strconv"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	gogithub "github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"

	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

// RepoInfo is a minimal repository description returned during installation syncs.
type RepoInfo struct {
	Owner string
	Name  string
}

// CreateAppJWT creates a short-lived JWT for GitHub App-level authentication (RS256).
func CreateAppJWT(appID int64, privateKeyPEM []byte) (string, error) {
	key, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return "", fmt.Errorf("github app jwt: %w", err)
	}
	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now.Add(-30 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
		Issuer:    strconv.FormatInt(appID, 10),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return tok.SignedString(key)
}

// VerifyAppCredentials confirms the App ID and private key are valid by signing
// an App JWT and calling GitHub's GET /app endpoint (the App's own metadata).
// This genuinely validates the credentials with GitHub — a structurally valid
// key paired with a wrong App ID will fail here — and returns the App's slug.
func VerifyAppCredentials(ctx context.Context, appID int64, privateKeyPEM []byte) (string, error) {
	appJWT, err := CreateAppJWT(appID, privateKeyPEM)
	if err != nil {
		return "", err
	}
	appClient := newRawClient(ctx, appJWT)
	// An empty slug targets GET /app — the currently authenticated App.
	start := time.Now()
	app, _, err := appClient.Apps.Get(ctx, "")
	logger.ExternalCall(ctx, "github", "Apps.Get", start, err, "app_id", appID)
	if err != nil {
		return "", fmt.Errorf("github app: verify credentials: %w", err)
	}
	return app.GetSlug(), nil
}

// NewInstallationClient exchanges an App JWT for an installation access token and
// returns a Client scoped to that installation.
func NewInstallationClient(ctx context.Context, appID int64, privateKeyPEM []byte, installationID int64) (Client, error) {
	appJWT, err := CreateAppJWT(appID, privateKeyPEM)
	if err != nil {
		return nil, err
	}
	appClient := newRawClient(ctx, appJWT)
	start := time.Now()
	instToken, _, err := appClient.Apps.CreateInstallationToken(ctx, installationID, nil)
	logger.ExternalCall(ctx, "github", "Apps.CreateInstallationToken", start, err, "app_id", appID, "installation_id", installationID)
	if err != nil {
		return nil, fmt.Errorf("github app: create installation token: %w", err)
	}
	return NewClient(instToken.GetToken()), nil
}

// ListInstallationRepos returns all repositories accessible to a given installation.
// It exchanges the App JWT for an installation access token, then lists repos.
func ListInstallationRepos(ctx context.Context, appID int64, privateKeyPEM []byte, installationID int64) ([]RepoInfo, error) {
	appJWT, err := CreateAppJWT(appID, privateKeyPEM)
	if err != nil {
		return nil, err
	}
	appClient := newRawClient(ctx, appJWT)

	// Exchange app JWT for an installation-scoped access token.
	tokStart := time.Now()
	instToken, _, err := appClient.Apps.CreateInstallationToken(ctx, installationID, nil)
	logger.ExternalCall(ctx, "github", "Apps.CreateInstallationToken", tokStart, err, "app_id", appID, "installation_id", installationID)
	if err != nil {
		return nil, fmt.Errorf("github app: create installation token: %w", err)
	}

	// Use installation token to enumerate accessible repos.
	instClient := newRawClient(ctx, instToken.GetToken())
	var repos []RepoInfo
	opts := &gogithub.ListOptions{PerPage: 100}
	for {
		listStart := time.Now()
		page, resp, err := instClient.Apps.ListRepos(ctx, opts)
		logger.ExternalCall(ctx, "github", "Apps.ListRepos", listStart, err, "installation_id", installationID, "page", opts.Page)
		if err != nil {
			return nil, fmt.Errorf("github app: list repos: %w", err)
		}
		for _, r := range page.Repositories {
			owner, name := splitFullName(r.GetFullName())
			repos = append(repos, RepoInfo{Owner: owner, Name: name})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return repos, nil
}

// InstallationInfo is a minimal description of a GitHub App installation.
type InstallationInfo struct {
	ID           int64
	AccountLogin string
	AccountType  string
}

// FindFirstInstallation calls GET /app/installations and returns the first result.
// Used during initial setup when the installation.created webhook was missed.
func FindFirstInstallation(ctx context.Context, appID int64, privateKeyPEM []byte) (*InstallationInfo, error) {
	appJWT, err := CreateAppJWT(appID, privateKeyPEM)
	if err != nil {
		return nil, err
	}
	appClient := newRawClient(ctx, appJWT)
	start := time.Now()
	installs, _, err := appClient.Apps.ListInstallations(ctx, &gogithub.ListOptions{PerPage: 1})
	logger.ExternalCall(ctx, "github", "Apps.ListInstallations", start, err, "app_id", appID)
	if err != nil {
		return nil, fmt.Errorf("github app: list installations: %w", err)
	}
	if len(installs) == 0 {
		return nil, fmt.Errorf("github app: no installations found — install the GitHub App on your organization")
	}
	i := installs[0]
	return &InstallationInfo{
		ID:           i.GetID(),
		AccountLogin: i.GetAccount().GetLogin(),
		AccountType:  i.GetAccount().GetType(),
	}, nil
}

// newRawClient creates a raw go-github client authenticated with the given token.
func newRawClient(ctx context.Context, token string) *gogithub.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return gogithub.NewClient(tc)
}

func splitFullName(fullName string) (owner, name string) {
	if i := strings.IndexByte(fullName, '/'); i >= 0 {
		return fullName[:i], fullName[i+1:]
	}
	return "", fullName
}

func parseRSAPrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS8 key is not RSA")
		}
		return rsaKey, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}
}
