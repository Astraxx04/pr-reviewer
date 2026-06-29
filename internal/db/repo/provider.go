package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
)

type ProviderRepo struct{ db *gorm.DB }

func NewProviderRepo(db *gorm.DB) *ProviderRepo { return &ProviderRepo{db: db} }

func (r *ProviderRepo) List(ctx context.Context, installationID uint) ([]models.ProviderConfig, error) {
	var providers []models.ProviderConfig
	err := r.db.WithContext(ctx).
		Where("installation_id = ? OR installation_id = 0", installationID).
		Find(&providers).Error
	return providers, err
}

func (r *ProviderRepo) Create(ctx context.Context, p *models.ProviderConfig) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *ProviderRepo) Update(ctx context.Context, p *models.ProviderConfig) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *ProviderRepo) Delete(ctx context.Context, id, installationID uint) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND (installation_id = ? OR installation_id = 0)", id, installationID).
		Delete(&models.ProviderConfig{}).Error
}

func (r *ProviderRepo) FindByID(ctx context.Context, id, installationID uint) (*models.ProviderConfig, error) {
	var p models.ProviderConfig
	err := r.db.WithContext(ctx).
		Where("id = ? AND (installation_id = ? OR installation_id = 0)", id, installationID).
		First(&p).Error
	return &p, err
}
