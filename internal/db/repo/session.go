package repo

import (
	"context"
	"time"

	"gorm.io/gorm"

	"pr-reviewer/internal/db/models"
)

type SessionRepo struct{ db *gorm.DB }

func NewSessionRepo(db *gorm.DB) *SessionRepo { return &SessionRepo{db: db} }

func (r *SessionRepo) Create(ctx context.Context, s *models.Session) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *SessionRepo) FindActive(ctx context.Context, id string) (*models.Session, error) {
	var s models.Session
	err := r.db.WithContext(ctx).
		Where("id = ? AND expires_at > ?", id, time.Now()).
		First(&s).Error
	return &s, err
}

func (r *SessionRepo) ListForUser(ctx context.Context, userID uint) ([]models.Session, error) {
	var sessions []models.Session
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND expires_at > ?", userID, time.Now()).
		Order("created_at desc").
		Find(&sessions).Error
	return sessions, err
}

func (r *SessionRepo) Delete(ctx context.Context, id string, userID uint) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&models.Session{}).Error
}

func (r *SessionRepo) DeleteAllForUser(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&models.Session{}).Error
}

func (r *SessionRepo) PruneExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&models.Session{}).Error
}
