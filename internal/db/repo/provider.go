package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
)

type ProviderRepo struct{ db *gorm.DB }

func NewProviderRepo(db *gorm.DB) *ProviderRepo { return &ProviderRepo{db: db} }

func (r *ProviderRepo) List(ctx context.Context) ([]models.ProviderConfig, error) {
	var providers []models.ProviderConfig
	err := r.db.WithContext(ctx).Find(&providers).Error
	return providers, err
}

func (r *ProviderRepo) Create(ctx context.Context, p *models.ProviderConfig) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *ProviderRepo) Update(ctx context.Context, p *models.ProviderConfig) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *ProviderRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.ProviderConfig{}, id).Error
}

func (r *ProviderRepo) FindByID(ctx context.Context, id uint) (*models.ProviderConfig, error) {
	var p models.ProviderConfig
	err := r.db.WithContext(ctx).First(&p, id).Error
	return &p, err
}
