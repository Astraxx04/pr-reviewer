package repo

import (
	"context"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"gorm.io/gorm"
)

type ReviewRepo struct {
	db *gorm.DB
}

func NewReviewRepo(db *gorm.DB) *ReviewRepo {
	return &ReviewRepo{db: db}
}

func (r *ReviewRepo) Create(ctx context.Context, review *models.Review) error {
	return r.db.WithContext(ctx).Create(review).Error
}

func (r *ReviewRepo) FindByID(ctx context.Context, id uint) (*models.Review, error) {
	var review models.Review
	err := r.db.WithContext(ctx).Preload("Comments").First(&review, id).Error
	return &review, err
}

func (r *ReviewRepo) ListByRepo(ctx context.Context, repoID uint, page, perPage int) ([]models.Review, int64, error) {
	var reviews []models.Review
	var total int64

	q := r.db.WithContext(ctx).
		Joins("JOIN pull_requests ON pull_requests.id = reviews.pr_id").
		Where("pull_requests.repo_id = ?", repoID)

	if err := q.Model(&models.Review{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("reviews.created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&reviews).Error
	return reviews, total, err
}
