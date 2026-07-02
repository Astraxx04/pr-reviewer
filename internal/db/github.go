package db

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	gh "github.com/Astraxx04/pr-reviewer/internal/github"
)

// ResolveInstallationClient loads the GitHub App credentials from the database,
// decrypts the private key, finds the single installation record, and returns
// an authenticated GitHub Client via the provided token cache.
//
// This is the standard way for any DB-backed code path to obtain a GitHub client
// without storing a global PAT. Call it at the top of each job or HTTP handler
// that needs GitHub API access.
func ResolveInstallationClient(ctx context.Context, gdb *gorm.DB, encKey string, cache *gh.InstallationTokenCache) (gh.Client, error) {
	var appCfg models.GithubAppConfig
	if err := gdb.WithContext(ctx).First(&appCfg).Error; err != nil {
		return nil, fmt.Errorf("github app not configured: %w", err)
	}
	privateKey, err := Decrypt(appCfg.PrivateKeyEncrypted, encKey)
	if err != nil {
		return nil, fmt.Errorf("key decryption failed: %w", err)
	}
	var inst models.Installation
	if err := gdb.WithContext(ctx).Order("id ASC").First(&inst).Error; err != nil {
		return nil, fmt.Errorf("no installation found — install the GitHub App on your organization: %w", err)
	}
	if inst.GithubInstallationID == nil {
		return nil, fmt.Errorf("installation not yet registered — reinstall the GitHub App")
	}
	return cache.GetOrRefresh(ctx, appCfg.AppID, []byte(privateKey), *inst.GithubInstallationID)
}
