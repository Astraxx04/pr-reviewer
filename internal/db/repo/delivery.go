package repo

import (
	"context"
	"errors"
	"time"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
	"gorm.io/gorm"
)

type DeliveryRepo struct {
	db *gorm.DB
}

func NewDeliveryRepo(db *gorm.DB) *DeliveryRepo {
	return &DeliveryRepo{db: db}
}

func (r *DeliveryRepo) IsProcessed(ctx context.Context, deliveryID string) bool {
	err := r.db.WithContext(ctx).
		First(&models.WebhookDelivery{}, "delivery_id = ?", deliveryID).Error
	return err == nil
}

func (r *DeliveryRepo) MarkProcessed(ctx context.Context, deliveryID string) error {
	return r.RecordDelivery(ctx, &models.WebhookDelivery{DeliveryID: deliveryID})
}

// RecordDelivery creates a delivery record with extended metadata. Duplicate key errors are silently ignored.
func (r *DeliveryRepo) RecordDelivery(ctx context.Context, d *models.WebhookDelivery) error {
	if d.ProcessedAt.IsZero() {
		d.ProcessedAt = time.Now()
	}
	result := r.db.WithContext(ctx).Create(d)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return result.Error
	}
	return nil
}

// PurgeOlderThan deletes webhook delivery records older than the given cutoff.
func (r *DeliveryRepo) PurgeOlderThan(ctx context.Context, cutoff time.Time) error {
	return r.db.WithContext(ctx).
		Where("processed_at < ?", cutoff).
		Delete(&models.WebhookDelivery{}).Error
}

// List returns paginated delivery records ordered by most recent first.
func (r *DeliveryRepo) List(ctx context.Context, limit, offset int) ([]models.WebhookDelivery, int64, error) {
	var rows []models.WebhookDelivery
	var total int64
	r.db.WithContext(ctx).Model(&models.WebhookDelivery{}).Count(&total)
	err := r.db.WithContext(ctx).
		Order("processed_at desc").
		Limit(limit).Offset(offset).
		Find(&rows).Error
	return rows, total, err
}
