package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
)

type AssignmentRuleRepo struct{ db *gorm.DB }

func NewAssignmentRuleRepo(db *gorm.DB) *AssignmentRuleRepo {
	return &AssignmentRuleRepo{db: db}
}

func (r *AssignmentRuleRepo) List(ctx context.Context, repoID uint) ([]models.AssignmentRule, error) {
	var rules []models.AssignmentRule
	err := r.db.WithContext(ctx).Where("repo_id = ?", repoID).Find(&rules).Error
	return rules, err
}

func (r *AssignmentRuleRepo) Create(ctx context.Context, rule *models.AssignmentRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *AssignmentRuleRepo) Delete(ctx context.Context, id, repoID uint) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND repo_id = ?", id, repoID).
		Delete(&models.AssignmentRule{}).Error
}
