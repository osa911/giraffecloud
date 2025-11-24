package service

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/osa911/giraffecloud/internal/db/ent"
	entplan "github.com/osa911/giraffecloud/internal/db/ent/plan"
	entusage "github.com/osa911/giraffecloud/internal/db/ent/usage"
	entuser "github.com/osa911/giraffecloud/internal/db/ent/user"
)

type QuotaDecision string

const (
	QuotaAllow QuotaDecision = "allow"
	QuotaWarn  QuotaDecision = "warn"
	QuotaBlock QuotaDecision = "block"
)

type QuotaResult struct {
	Decision   QuotaDecision
	UsedBytes  int64
	LimitBytes int64
}

type QuotaService interface {
	CheckUser(ctx context.Context, userID uint32) (QuotaResult, error)
}

type quotaService struct {
	db               *ent.Client
	defaultLimit     int64
	warnThresholdPct float64

	mu    sync.RWMutex
	cache map[uint32]quotaCacheEntry
}

type quotaCacheEntry struct {
	result QuotaResult
	at     time.Time
}

func NewQuotaService(db *ent.Client) QuotaService {
	limit := parseInt64Env("QUOTA_DEFAULT_MONTHLY_BYTES", 100*1024*1024*1024) // 100 GB default
	warnPct := parseFloatEnv("QUOTA_SOFT_WARN_PCT", 0.9)
	return &quotaService{
		db:               db,
		defaultLimit:     limit,
		warnThresholdPct: warnPct,
		cache:            make(map[uint32]quotaCacheEntry),
	}
}

func (s *quotaService) CheckUser(ctx context.Context, userID uint32) (QuotaResult, error) {
	// unlimited
	if s.defaultLimit <= 0 {
		return QuotaResult{Decision: QuotaAllow, UsedBytes: 0, LimitBytes: 0}, nil
	}

	// 30s cache with read lock for better performance
	s.mu.RLock()
	if entry, ok := s.cache[userID]; ok && time.Since(entry.at) < 30*time.Second {
		res := entry.result
		s.mu.RUnlock()
		return res, nil
	}
	s.mu.RUnlock()

	// Determine plan limit for user (fallback to default)
	limit := s.defaultLimit
	if u, err := s.db.User.Query().
		Where(entuser.ID(userID)).
		Only(ctx); err == nil && u != nil {
		if u.PlanName != nil && *u.PlanName != "" {
			if p, err := s.db.Plan.Query().
				Where(entplan.NameEQ(*u.PlanName), entplan.ActiveEQ(true)).
				Only(ctx); err == nil && p != nil {
				limit = p.MonthlyLimitBytes
			}
		}
	}

	// Sum current month usage
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	rows, err := s.db.Usage.Query().
		Where(
			entusage.PeriodStartGTE(monthStart),
			entusage.UserIDEQ(userID),
		).All(ctx)
	if err != nil {
		// On error, fail open to avoid traffic loss
		return QuotaResult{Decision: QuotaAllow, UsedBytes: 0, LimitBytes: limit}, nil
	}

	var used int64
	for _, r := range rows {
		used += r.BytesIn + r.BytesOut
	}

	decision := QuotaAllow
	if float64(used) >= float64(limit)*s.warnThresholdPct && used < limit {
		decision = QuotaWarn
	}
	if used >= limit {
		decision = QuotaBlock
	}

	res := QuotaResult{Decision: decision, UsedBytes: used, LimitBytes: limit}
	s.mu.Lock()
	s.cache[userID] = quotaCacheEntry{result: res, at: time.Now()}

	// Clean up old cache entries to prevent memory leaks (keep only last 10k entries)
	if len(s.cache) > 10000 {
		// Remove oldest 20% of entries
		cutoff := time.Now().Add(-5 * time.Minute)
		for userID, entry := range s.cache {
			if entry.at.Before(cutoff) {
				delete(s.cache, userID)
			}
		}
	}
	s.mu.Unlock()
	return res, nil
}

func parseInt64Env(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func parseFloatEnv(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return def
}
