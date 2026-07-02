package handlers

import (
	"context"
	"strings"

	"gorm.io/gorm"

	dbpkg "github.com/Astraxx04/pr-reviewer/internal/db"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	gh "github.com/Astraxx04/pr-reviewer/internal/github"
)

// syncLoginRepoAccess updates repo_access rows for a single user at login time.
// It runs as a fire-and-forget goroutine; all errors are silently dropped because
// a sync failure must never block the login flow.
//
// For each enabled repo, it checks whether the user appears in the GitHub
// collaborators list and upserts or removes their access row accordingly.
// Only this user's rows are touched, so concurrent logins are safe.
func syncLoginRepoAccess(ctx context.Context, db *gorm.DB, encryptionKey, login string) {
	var appCfg models.GithubAppConfig
	if db.WithContext(ctx).First(&appCfg).Error != nil {
		return // GitHub App not configured yet
	}
	privateKey, err := dbpkg.Decrypt(appCfg.PrivateKeyEncrypted, encryptionKey)
	if err != nil {
		return
	}

	var inst models.Installation
	if db.WithContext(ctx).Order("id ASC").First(&inst).Error != nil || inst.GithubInstallationID == nil {
		return
	}

	ghClient, err := gh.NewInstallationClient(ctx, appCfg.AppID, []byte(privateKey), *inst.GithubInstallationID)
	if err != nil {
		return
	}

	var repos []models.Repository
	db.WithContext(ctx).Where("enabled = true").Find(&repos)

	for _, r := range repos {
		logins, err := ghClient.ListRepoCollaborators(ctx, r.Owner, r.Name)
		if err != nil {
			continue
		}

		hasAccess := false
		for _, l := range logins {
			if strings.EqualFold(l, login) {
				hasAccess = true
				break
			}
		}

		if hasAccess {
			db.WithContext(ctx).
				Where(models.RepoAccess{RepoID: r.ID, Login: login}).
				FirstOrCreate(&models.RepoAccess{RepoID: r.ID, Login: login})
		} else {
			db.WithContext(ctx).
				Where("repo_id = ? AND login = ?", r.ID, login).
				Delete(&models.RepoAccess{})
		}
	}
}
