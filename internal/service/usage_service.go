package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/osa911/giraffecloud/internal/db/ent"
	entusage "github.com/osa911/giraffecloud/internal/db/ent/usage"
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
	Start(ctx context.Context)
	Stop(ctx context.Context) error
}

type usageService struct {
	mu      sync.RWMutex            // Use RWMutex for better read performance
	records map[string]*UsageRecord // key: YYYY-MM-DD:user:tunnel

	// Performance optimizations for 100k users
	flushInterval time.Duration
	lastFlush     time.Time
	batchSize     int

	// Database client for background flushes
	dbClient *ent.Client

	// Lifecycle management
	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewUsageService() UsageService {
	return &usageService{
		records:       make(map[string]*UsageRecord),
		flushInterval: 30 * time.Second, // Flush every 30 seconds for better performance
		lastFlush:     time.Now(),
		batchSize:     1000, // Process 1000 records at a time
		stopChan:      make(chan struct{}),
	}
}

// NewUsageServiceWithDB creates a usage service with database client for background flushes
func NewUsageServiceWithDB(dbClient *ent.Client) UsageService {
	return &usageService{
		records:       make(map[string]*UsageRecord),
		flushInterval: 30 * time.Second,
		lastFlush:     time.Now(),
		batchSize:     1000,
		dbClient:      dbClient,
		stopChan:      make(chan struct{}),
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

	// Use write lock only when necessary
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

	// Auto-flush if we have too many records or time has passed
	if len(s.records) > s.batchSize || time.Since(s.lastFlush) > s.flushInterval {
		// Trigger background flush (non-blocking)
		go s.triggerBackgroundFlush()
	}
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
// Optimized for 100k users with batch processing and connection pooling.
func (s *usageService) FlushToDB(ctx context.Context, client *ent.Client) error {
	records := s.SnapshotAndReset()
	if len(records) == 0 {
		return nil
	}

	// Update last flush time
	s.mu.Lock()
	s.lastFlush = time.Now()
	s.mu.Unlock()

	// Process records in batches for better performance
	for i := 0; i < len(records); i += s.batchSize {
		end := i + s.batchSize
		if end > len(records) {
			end = len(records)
		}

		batch := records[i:end]
		if err := s.processBatch(ctx, client, batch); err != nil {
			return fmt.Errorf("failed to process batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// processBatch processes a batch of usage records with optimized database operations
func (s *usageService) processBatch(ctx context.Context, client *ent.Client, records []UsageRecord) error {
	// Use a transaction for better performance and consistency
	tx, err := client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if v := recover(); v != nil {
			tx.Rollback()
			panic(v)
		}
	}()

	for _, r := range records {
		existing, err := tx.Usage.Query().
			Where(
				entusage.PeriodStartEQ(r.PeriodStart),
				entusage.UserIDEQ(r.UserID),
				entusage.TunnelIDEQ(r.TunnelID),
				entusage.DomainEQ(r.Domain),
			).
			Only(ctx)
		if err == nil && existing != nil {
			// Update existing record
			if _, err := tx.Usage.UpdateOneID(existing.ID).
				SetBytesIn(existing.BytesIn + r.BytesIn).
				SetBytesOut(existing.BytesOut + r.BytesOut).
				SetRequests(existing.Requests + r.Requests).
				Save(ctx); err != nil {
				tx.Rollback()
				return err
			}
		} else {
			// Create new record
			if _, err := tx.Usage.Create().
				SetPeriodStart(r.PeriodStart).
				SetUserID(r.UserID).
				SetTunnelID(r.TunnelID).
				SetDomain(r.Domain).
				SetBytesIn(r.BytesIn).
				SetBytesOut(r.BytesOut).
				SetRequests(r.Requests).
				Save(ctx); err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	return tx.Commit()
}

// triggerBackgroundFlush performs a non-blocking background flush
func (s *usageService) triggerBackgroundFlush() {
	// Create a context with timeout for the background flush
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get a snapshot of current records without blocking the main thread
	s.mu.RLock()
	recordCount := len(s.records)
	shouldFlush := recordCount > s.batchSize || time.Since(s.lastFlush) > s.flushInterval
	s.mu.RUnlock()

	if !shouldFlush {
		return // Another goroutine already handled it
	}

	// Perform the flush in background if database client is available
	if s.dbClient != nil {
		if err := s.FlushToDB(ctx, s.dbClient); err != nil {
			// Log error but don't block the main thread
			// In production, you'd use a proper logger
			fmt.Printf("Background usage flush failed: %v\n", err)
		}

	}

	// Update the last flush time to prevent immediate re-triggering
	s.mu.Lock()
	s.lastFlush = time.Now()
	s.mu.Unlock()
}

// Start starts the background flush task
func (s *usageService) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.flushInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if s.dbClient != nil {
					if err := s.FlushToDB(context.Background(), s.dbClient); err != nil {
						fmt.Printf("Periodic usage flush failed: %v\n", err)
					}
				}
			case <-s.stopChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop stops the background flush task and performs a final flush
func (s *usageService) Stop(ctx context.Context) error {
	close(s.stopChan)
	s.wg.Wait()

	// Final flush
	if s.dbClient != nil {
		return s.FlushToDB(ctx, s.dbClient)
	}
	return nil
}
