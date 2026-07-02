package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/Astraxx04/pr-reviewer/internal/db/models"
)

// ListTeamLogins returns the GitHub logins of all active admin and reviewer users.
// Used by the assignment evaluator when no explicit member list is configured on a rule.
func ListTeamLogins(ctx context.Context, db *gorm.DB) ([]string, error) {
	var users []models.User
	err := db.WithContext(ctx).
		Where("role IN ? AND status = ?", []string{"admin", "reviewer"}, "active").
		Select("login").
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	logins := make([]string, 0, len(users))
	for _, u := range users {
		logins = append(logins, u.Login)
	}
	return logins, nil
}
