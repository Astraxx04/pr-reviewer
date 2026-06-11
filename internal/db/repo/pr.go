package repo

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"pr-reviewer/internal/db/models"
)

type PRRepo struct {
	db *gorm.DB
}

func NewPRRepo(db *gorm.DB) *PRRepo {
	return &PRRepo{db: db}
}

// Upsert finds or creates a PullRequest row by (repo_id, number).
func (r *PRRepo) Upsert(ctx context.Context, pr *models.PullRequest) error {
	return r.db.WithContext(ctx).
		Where(models.PullRequest{RepoID: pr.RepoID, Number: pr.Number}).
		Assign(models.PullRequest{Title: pr.Title, Author: pr.Author, HeadSHA: pr.HeadSHA}).
		FirstOrCreate(pr).Error
}

// UpsertInstallationStub finds the installation for an account login, creating a
// stub (with no GitHub installation ID yet) when none exists. account_login is
// the natural key — one installation per account — so concurrent callers
// converge on a single row. The real GithubInstallationID is filled in later by
// the installation.created webhook (see ReconcileInstallation).
func UpsertInstallationStub(ctx context.Context, db *gorm.DB, login, accountType string) (*models.Installation, error) {
	var inst models.Installation
	err := db.WithContext(ctx).
		Where(models.Installation{AccountLogin: login}).
		Attrs(models.Installation{AccountType: accountType}).
		FirstOrCreate(&inst).Error
	return &inst, err
}

// UpsertRepository finds a repository by its globally-unique (owner, name) key,
// creating it under the given installation when absent. Existing rows keep
// their current installation and enabled flag — enabledIfNew applies only on
// creation. The bool result reports whether a new row was created.
func UpsertRepository(ctx context.Context, db *gorm.DB, installationID uint, owner, name string, enabledIfNew bool) (*models.Repository, bool, error) {
	var repo models.Repository
	tx := db.WithContext(ctx).
		Where(models.Repository{Owner: owner, Name: name}).
		Attrs(models.Repository{InstallationID: installationID, Enabled: enabledIfNew}).
		FirstOrCreate(&repo)
	return &repo, tx.RowsAffected == 1, tx.Error
}

// FindOrCreateRepo ensures an Installation + Repository row exist for the given owner/name.
// When triggered by a webhook there is no Installation record yet, so we create a stub.
func FindOrCreateRepo(ctx context.Context, db *gorm.DB, owner, name string) (*models.Repository, error) {
	inst, err := UpsertInstallationStub(ctx, db, owner, "User")
	if err != nil {
		return nil, err
	}
	repo, _, err := UpsertRepository(ctx, db, inst.ID, owner, name, true)
	return repo, err
}

// UpsertOnConflict uses ON CONFLICT DO UPDATE for PullRequest rows.
func (r *PRRepo) UpsertOnConflict(ctx context.Context, pr *models.PullRequest) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "repo_id"}, {Name: "number"}},
			DoUpdates: clause.AssignmentColumns([]string{"title", "author", "head_sha"}),
		}).
		Create(pr).Error
}
