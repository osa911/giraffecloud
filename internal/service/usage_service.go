package service

import (
	"context"
	"sync"
	"time"

	"giraffecloud/internal/db/ent"
	entusage "giraffecloud/internal/db/ent/usage"
)

// UsageRecord represents aggregated usage for a period.
type UsageRecord struct {
	PeriodStart time.Time
	UserID      uint32
	TunnelID    uint32
	Domain      string
	BytesIn     int64
	BytesOut    int64
	Requests    int64
}

// UsageService provides in-memory aggregation of traffic usage.
// This can be later extended to persist to a database.
type UsageService interface {
	Increment(userID uint32, tunnelID uint32, domain string, bytesIn int64, bytesOut int64, requests int64)
	SnapshotAndReset() []UsageRecord
	FlushToDB(ctx context.Context, client *ent.Client) error
}

type usageService struct {
	mu      sync.Mutex
	records map[string]*UsageRecord // key: YYYY-MM-DD:user:tunnel
}

func NewUsageService() UsageService {
	return &usageService{
		records: make(map[string]*UsageRecord),
	}
}

func (s *usageService) key(day string, userID, tunnelID uint32) string {
	return day + ":" + fmtUint32(userID) + ":" + fmtUint32(tunnelID)
}

func (s *usageService) Increment(userID uint32, tunnelID uint32, domain string, bytesIn int64, bytesOut int64, requests int64) {
	// Aggregate by UTC day
	now := time.Now().UTC()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dayKey := day.Format("2006-01-02")

	s.mu.Lock()
	defer s.mu.Unlock()

	k := s.key(dayKey, userID, tunnelID)
	rec, ok := s.records[k]
	if !ok {
		rec = &UsageRecord{PeriodStart: day, UserID: userID, TunnelID: tunnelID, Domain: domain}
		s.records[k] = rec
	}
	rec.BytesIn += bytesIn
	rec.BytesOut += bytesOut
	rec.Requests += requests
}

func (s *usageService) SnapshotAndReset() []UsageRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]UsageRecord, 0, len(s.records))
	for _, rec := range s.records {
		out = append(out, *rec)
	}
	s.records = make(map[string]*UsageRecord)
	return out
}

// Helper to avoid fmt import; small fast formatter for uint32.
func fmtUint32(v uint32) string {
	// Maximum length for uint32 is 10 digits
	var buf [10]byte
	i := len(buf)
	if v == 0 {
		return "0"
	}
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

// FlushToDB persists the current snapshot to the database (upsert by period/user/tunnel/domain).
func (s *usageService) FlushToDB(ctx context.Context, client *ent.Client) error {
	records := s.SnapshotAndReset()
	for i := range records {
		r := records[i]
		existing, err := client.Usage.Query().
			Where(
				entusage.PeriodStartEQ(r.PeriodStart),
				entusage.UserIDEQ(r.UserID),
				entusage.TunnelIDEQ(r.TunnelID),
				entusage.DomainEQ(r.Domain),
			).
			Only(ctx)
		if err == nil && existing != nil {
			if _, err := client.Usage.UpdateOneID(existing.ID).
				SetBytesIn(existing.BytesIn + r.BytesIn).
				SetBytesOut(existing.BytesOut + r.BytesOut).
				SetRequests(existing.Requests + r.Requests).
				Save(ctx); err != nil {
				return err
			}
			continue
		}
		if _, err := client.Usage.Create().
			SetPeriodStart(r.PeriodStart).
			SetUserID(r.UserID).
			SetTunnelID(r.TunnelID).
			SetDomain(r.Domain).
			SetBytesIn(r.BytesIn).
			SetBytesOut(r.BytesOut).
			SetRequests(r.Requests).
			Save(ctx); err != nil {
			return err
		}
	}
	return nil
}
