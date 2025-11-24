package service

import (
	"context"
	"fmt"

	"github.com/osa911/giraffecloud/internal/db/ent"
	entplan "github.com/osa911/giraffecloud/internal/db/ent/plan"
	"github.com/osa911/giraffecloud/internal/logging"
)

type PlanService struct {
	db     *ent.Client
	logger *logging.Logger
}

func NewPlanService(db *ent.Client) *PlanService {
	return &PlanService{db: db, logger: logging.GetGlobalLogger()}
}

// SeedDefaultPlans inserts default plans if they don't exist yet.
func (s *PlanService) SeedDefaultPlans(ctx context.Context) error {
	defaults := []struct {
		Name              string
		MonthlyLimitBytes int64
		OveragePerGBCents int
	}{
		{Name: "Free", MonthlyLimitBytes: 10 * 1024 * 1024 * 1024, OveragePerGBCents: 0},         // 10 GB
		{Name: "Pro", MonthlyLimitBytes: 100 * 1024 * 1024 * 1024, OveragePerGBCents: 200},       // 100 GB, $2/GB
		{Name: "Business", MonthlyLimitBytes: 1024 * 1024 * 1024 * 1024, OveragePerGBCents: 150}, // 1 TB, $1.5/GB
	}

	for _, p := range defaults {
		exists, err := s.db.Plan.Query().Where(entplan.NameEQ(p.Name)).Exist(ctx)
		if err != nil {
			s.logger.Warn("Failed checking plan %s existence: %v", p.Name, err)
			continue
		}
		if exists {
			continue
		}
		if _, err := s.db.Plan.Create().
			SetName(p.Name).
			SetMonthlyLimitBytes(p.MonthlyLimitBytes).
			SetOveragePerGBCents(p.OveragePerGBCents).
			SetActive(true).
			Save(ctx); err != nil {
			s.logger.Warn("Failed seeding plan %s: %v", p.Name, err)
			return fmt.Errorf("seed plan %s: %w", p.Name, err)
		}
		s.logger.Info("Seeded plan: %s", p.Name)
	}
	return nil
}
