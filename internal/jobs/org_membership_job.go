package jobs

import (
	"context"
	"strings"

	gogithub "github.com/google/go-github/v69/github"
	"github.com/riverqueue/river"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	dbpkg "github.com/Astraxx04/pr-reviewer/internal/db"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	gh "github.com/Astraxx04/pr-reviewer/internal/github"
	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

// OrgMembershipCheckJobArgs is the River job payload.
type OrgMembershipCheckJobArgs struct{}

func (OrgMembershipCheckJobArgs) Kind() string { return "org_membership_check" }

// OrgMembershipCheckWorker periodically verifies that every active platform user
// is still a member of the required GitHub organisation. Users who have left (or
// been removed from) the org are suspended immediately and their sessions wiped.
//
// Requires the GitHub App to have "Organization members: Read-only" permission.
// If the API call fails for any user, that user is skipped — the job is best-effort;
// the hard gate remains the login-time IsMember check.
type OrgMembershipCheckWorker struct {
	river.WorkerDefaults[OrgMembershipCheckJobArgs]
	DB            *gorm.DB
	Log           *logger.Logger
	RequiredOrg   string
	EncryptionKey string
}

func (w *OrgMembershipCheckWorker) Work(ctx context.Context, job *river.Job[OrgMembershipCheckJobArgs]) error {
	if w.RequiredOrg == "" {
		return nil
	}

	// Build an installation-scoped GitHub client.
	var appCfg models.GithubAppConfig
	if w.DB.WithContext(ctx).First(&appCfg).Error != nil {
		w.Log.Warn("org membership check: GitHub App not configured, skipping")
		return nil
	}
	privateKey, err := dbpkg.Decrypt(appCfg.PrivateKeyEncrypted, w.EncryptionKey)
	if err != nil {
		w.Log.Warn("org membership check: key decrypt failed", "error", err)
		return nil
	}

	var inst models.Installation
	if w.DB.WithContext(ctx).Order("id ASC").First(&inst).Error != nil || inst.GithubInstallationID == nil {
		w.Log.Warn("org membership check: no installation found, skipping")
		return nil
	}

	// Exchange App JWT for an installation access token, then build a raw go-github
	// client to call Organizations.IsMember (requires members:read org permission on the App).
	appJWT, err := gh.CreateAppJWT(appCfg.AppID, []byte(privateKey))
	if err != nil {
		w.Log.Warn("org membership check: App JWT creation failed", "error", err)
		return nil
	}
	appClient := gogithub.NewClient(
		oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: appJWT})),
	)
	instTokenResp, _, err := appClient.Apps.CreateInstallationToken(ctx, *inst.GithubInstallationID, nil)
	if err != nil {
		w.Log.Warn("org membership check: installation token failed", "error", err)
		return nil
	}
	ghClient := gogithub.NewClient(
		oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: instTokenResp.GetToken()})),
	)

	// Fetch all active users.
	var users []models.User
	w.DB.WithContext(ctx).Where("status = ?", "active").Find(&users)

	suspended := 0
	for _, u := range users {
		isMember, resp, err := ghClient.Organizations.IsMember(ctx, w.RequiredOrg, u.Login)
		if err != nil {
			// 404 means definitively not a member; other errors → skip to avoid false positives.
			if resp == nil || resp.StatusCode != 404 {
				w.Log.Warn("org membership check: API error, skipping user", "login", u.Login, "error", err)
				continue
			}
		}
		if !isMember || (err != nil && strings.Contains(err.Error(), "404")) {
			w.DB.WithContext(ctx).Model(&u).Update("status", "suspended")
			w.DB.WithContext(ctx).Where("user_id = ?", u.ID).Delete(&models.Session{})
			w.Log.Info("org membership check: suspended user (no longer org member)", "login", u.Login)
			suspended++
		}
	}

	if suspended > 0 {
		w.Log.Info("org membership check complete", "suspended", suspended, "checked", len(users))
	}
	return nil
}
