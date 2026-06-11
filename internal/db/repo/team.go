package repo

import (
	"context"

	"gorm.io/gorm"

	"pr-reviewer/internal/db/models"
)

type TeamMemberRepo struct{ db *gorm.DB }

func NewTeamMemberRepo(db *gorm.DB) *TeamMemberRepo { return &TeamMemberRepo{db: db} }

func (r *TeamMemberRepo) List(ctx context.Context, installationID uint) ([]models.TeamMember, error) {
	var members []models.TeamMember
	err := r.db.WithContext(ctx).Where("installation_id = ?", installationID).Find(&members).Error
	return members, err
}
