package repo

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"pr-reviewer/internal/db/models"
)

type UserRepo struct{ db *gorm.DB }

func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) Upsert(ctx context.Context, u *models.User) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "github_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"login", "email", "avatar_url"}),
		}).
		Create(u).Error
}

func (r *UserRepo) FindByGithubID(ctx context.Context, githubID int64) (*models.User, error) {
	var u models.User
	err := r.db.WithContext(ctx).Where("github_id = ?", githubID).First(&u).Error
	return &u, err
}

func (r *UserRepo) FindByLogin(ctx context.Context, login string) (*models.User, error) {
	var u models.User
	err := r.db.WithContext(ctx).Where("login = ?", login).First(&u).Error
	return &u, err
}

func (r *UserRepo) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.User{}).Count(&n).Error
	return n, err
}

func (r *UserRepo) FindAll(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := r.db.WithContext(ctx).Order("created_at asc").Find(&users).Error
	return users, err
}

func (r *UserRepo) UpdateRole(ctx context.Context, id uint, role string) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Update("role", role).Error
}

func (r *UserRepo) UpdateStatus(ctx context.Context, id uint, status string) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Update("status", status).Error
}

func (r *UserRepo) FindByID(ctx context.Context, id uint) (*models.User, error) {
	var u models.User
	err := r.db.WithContext(ctx).First(&u, id).Error
	return &u, err
}
