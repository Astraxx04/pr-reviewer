package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/audit"
	dbpkg "github.com/Astraxx04/pr-reviewer/internal/db"
	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"github.com/Astraxx04/pr-reviewer/internal/db/repo"
	gh "github.com/Astraxx04/pr-reviewer/internal/github"
	"github.com/Astraxx04/pr-reviewer/internal/jobs"
)

type RepoHandler struct {
	db            *gorm.DB
	encryptionKey string
	enqueuer      jobs.JobEnqueuer // optional; nil if RAG/indexing is disabled
}

func NewRepoHandler(db *gorm.DB, encryptionKey string) *RepoHandler {
	return &RepoHandler{db: db, encryptionKey: encryptionKey}
}

// WithEnqueuer attaches a job enqueuer so the handler can trigger indexing jobs.
func (h *RepoHandler) WithEnqueuer(e jobs.JobEnqueuer) *RepoHandler {
	h.enqueuer = e
	return h
}

func (h *RepoHandler) List(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	q := h.db.WithContext(r.Context())
	if !isAdmin(user) {
		// Strictly fail-closed: non-admins see only repos with an explicit access row.
		// Repos not yet synced are invisible until sync runs (triggered at login).
		q = q.Where("id IN (SELECT repo_id FROM repo_accesses WHERE login = ?)", user.Login)
	}
	var repos []models.Repository
	q.Find(&repos)
	writeJSON(w, http.StatusOK, repos)
}

// canAccessRepo reports whether the user may view the given repo.
// Admins/owners always can. Non-admins require an explicit repo_access row (fail-closed).
func (h *RepoHandler) canAccessRepo(r *http.Request, repoID uint) bool {
	user := getUser(r)
	if user == nil {
		return false
	}
	if isAdmin(user) {
		return true
	}
	var mine int64
	h.db.WithContext(r.Context()).Model(&models.RepoAccess{}).
		Where("repo_id = ? AND login = ?", repoID, user.Login).Count(&mine)
	return mine > 0
}

func (h *RepoHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	var before models.Repository
	h.db.WithContext(r.Context()).First(&before, id)

	if body.Enabled != nil {
		h.db.WithContext(r.Context()).Model(&models.Repository{}).Where("id = ?", id).Update("enabled", *body.Enabled)
	}

	var repo models.Repository
	h.db.WithContext(r.Context()).First(&repo, id)

	// When a repo is first enabled and RAG indexing is available, kick off full indexing.
	if body.Enabled != nil && *body.Enabled && !before.Enabled && h.enqueuer != nil {
		_, _ = h.enqueuer.Insert(r.Context(), jobs.IndexRepoJobArgs{
			Owner:  repo.Owner,
			Repo:   repo.Name,
			RepoID: repo.ID,
		}, nil)
	}

	writeJSON(w, http.StatusOK, repo)

	// Audit log for enable/disable actions.
	user := getUser(r)
	if user != nil && body.Enabled != nil {
		action := "repo.disable"
		if *body.Enabled {
			action = "repo.enable"
		}
		audit.Log(h.db, r, user.Login, user.ID, action, "repo",
			fmt.Sprint(id), map[string]any{"enabled": before.Enabled},
			map[string]any{"enabled": *body.Enabled})
	}
}

// Index triggers a full re-index of the repository on demand.
func (h *RepoHandler) Index(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if h.enqueuer == nil {
		writeError(w, http.StatusServiceUnavailable, "indexing not available (no embedding provider configured)")
		return
	}
	var repo models.Repository
	if err := h.db.WithContext(r.Context()).First(&repo, id).Error; err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if _, err := h.enqueuer.Insert(r.Context(), jobs.IndexRepoJobArgs{
		Owner:  repo.Owner,
		Repo:   repo.Name,
		RepoID: repo.ID,
	}, nil); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue index job")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *RepoHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	// Hide repos the user has no access to (return 404, not 403, so we don't leak existence).
	if !h.canAccessRepo(r, uint(id)) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var repo models.Repository
	if err := h.db.WithContext(r.Context()).First(&repo, id).Error; err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": repo.Config})
}

// Sync fetches repositories from the GitHub App installation and upserts them into the DB.
// It uses the stored GithubAppConfig credentials and the Installation record for the current user.
func (h *RepoHandler) Sync(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Load GitHub App config.
	var appCfg models.GithubAppConfig
	if err := h.db.First(&appCfg).Error; err != nil {
		writeError(w, http.StatusBadRequest, "GitHub App not configured — add credentials in Settings → GitHub App")
		return
	}
	privateKey, err := dbpkg.Decrypt(appCfg.PrivateKeyEncrypted, h.encryptionKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "key decryption failed")
		return
	}

	// Load the single installation record, auto-registering it if missing.
	// This handles the case where the installation.created webhook was never
	// received (e.g. the app was installed before the server was running).
	var inst models.Installation
	if err := h.db.Order("id ASC").First(&inst).Error; err != nil {
		info, discoverErr := gh.FindFirstInstallation(r.Context(), appCfg.AppID, []byte(privateKey))
		if discoverErr != nil {
			writeError(w, http.StatusNotFound, "no installation found — install the GitHub App on your organization and try again")
			return
		}
		inst = models.Installation{
			GithubInstallationID: &info.ID,
			AccountLogin:         info.AccountLogin,
			AccountType:          info.AccountType,
		}
		h.db.WithContext(r.Context()).
			Where(models.Installation{AccountLogin: info.AccountLogin}).
			Assign(models.Installation{GithubInstallationID: &info.ID, AccountType: info.AccountType}).
			FirstOrCreate(&inst)
	}

	if inst.GithubInstallationID == nil {
		writeError(w, http.StatusConflict, "installation is not fully registered yet — reinstall the GitHub App")
		return
	}

	repos, err := gh.ListInstallationRepos(r.Context(), appCfg.AppID, []byte(privateKey), *inst.GithubInstallationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GitHub API error: "+err.Error())
		return
	}

	// Installation client, reused to sync each repo's collaborators into repo_accesses.
	instClient, instErr := gh.NewInstallationClient(r.Context(), appCfg.AppID, []byte(privateKey), *inst.GithubInstallationID)

	added, accessSynced := 0, 0
	for _, ri := range repos {
		rec, created, err := repo.UpsertRepository(r.Context(), h.db, ri.Owner, ri.Name, false)
		if err != nil {
			continue
		}
		if created {
			added++
		}
		if instErr != nil || rec == nil {
			continue
		}
		// Sync who can see this repo. On error (e.g. the App lacks permission to
		// read collaborators) we leave existing access rows untouched rather than
		// wiping them — combined with the per-repo fail-open in List, that avoids
		// locking the team out of a repo we simply couldn't read.
		logins, cErr := instClient.ListRepoCollaborators(r.Context(), ri.Owner, ri.Name)
		if cErr != nil {
			continue
		}
		h.replaceRepoAccess(r, rec.ID, logins)
		accessSynced++
	}

	resp := map[string]any{"synced": len(repos), "added": added, "access_synced": accessSynced}
	if instErr != nil {
		resp["access_warning"] = "could not read repo collaborators — repo-level access control is inactive (check the GitHub App's permissions): " + instErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

// replaceRepoAccess overwrites the access rows for a repo with the given logins.
func (h *RepoHandler) replaceRepoAccess(r *http.Request, repoID uint, logins []string) {
	h.db.WithContext(r.Context()).Where("repo_id = ?", repoID).Delete(&models.RepoAccess{})
	if len(logins) == 0 {
		return
	}
	rows := make([]models.RepoAccess, 0, len(logins))
	for _, login := range logins {
		rows = append(rows, models.RepoAccess{RepoID: repoID, Login: login})
	}
	h.db.WithContext(r.Context()).Create(&rows)
}

func (h *RepoHandler) PutConfig(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	raw, _ := json.Marshal(body)
	h.db.WithContext(r.Context()).Model(&models.Repository{}).Where("id = ?", id).Update("config", raw)
	writeJSON(w, http.StatusOK, map[string]any{"config": body})
}
