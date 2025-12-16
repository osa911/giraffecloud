package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/osa911/giraffecloud/internal/db/ent"
	"github.com/osa911/giraffecloud/internal/db/ent/usage"
)

// UsageRepository handles usage-related database operations
type UsageRepository interface {
	GetDailyUsage(ctx context.Context, userID uint32, date time.Time) ([]*ent.Usage, error)
	GetUsageHistory(ctx context.Context, userID uint32, startDate time.Time) ([]*ent.Usage, error)
}

type usageRepository struct {
	client *ent.Client
}

// NewUsageRepository creates a new usage repository instance
func NewUsageRepository(client *ent.Client) UsageRepository {
	return &usageRepository{client: client}
}

// GetDailyUsage returns usage records for a specific day
func (r *usageRepository) GetDailyUsage(ctx context.Context, userID uint32, date time.Time) ([]*ent.Usage, error) {
	dayStart := date.Truncate(24 * time.Hour)
	records, err := r.client.Usage.Query().
		Where(
			usage.PeriodStartEQ(dayStart),
			usage.UserIDEQ(userID),
		).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch daily usage: %w", err)
	}
	return records, nil
}

// GetUsageHistory returns usage records starting from a specific date
func (r *usageRepository) GetUsageHistory(ctx context.Context, userID uint32, startDate time.Time) ([]*ent.Usage, error) {
	records, err := r.client.Usage.Query().
		Where(
			usage.UserIDEQ(userID),
			usage.PeriodStartGTE(startDate),
		).
		Order(ent.Asc(usage.FieldPeriodStart)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch usage history: %w", err)
	}
	return records, nil
}
