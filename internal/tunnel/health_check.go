package tunnel

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// HealthStatus represents the health state of a tunnel
type HealthStatus int

const (
	StatusUnknown HealthStatus = iota
	StatusHealthy
	StatusDegraded
	StatusUnhealthy
)

// HealthCheck configuration
type HealthCheckConfig struct {
	Interval          time.Duration
	Timeout           time.Duration
	RiseCount         int // Number of successful checks before marking as healthy
	FallCount         int // Number of failed checks before marking as unhealthy
	MaxResponseTime   time.Duration
	MaxRetries        int
	SuccessThreshold  float64 // Percentage of successful checks required
}

// DefaultHealthCheckConfig returns the default health check configuration
func DefaultHealthCheckConfig() *HealthCheckConfig {
	return &HealthCheckConfig{
		Interval:          time.Second * 5,
		Timeout:          time.Second * 3,
		RiseCount:        2,
		FallCount:        3,
		MaxResponseTime:  time.Second * 1,
		MaxRetries:       2,
		SuccessThreshold: 0.8, // 80% success rate required
	}
}

// HealthChecker manages health checks for a tunnel
type HealthChecker struct {
	config     *HealthCheckConfig
	status     HealthStatus
	statusMu   sync.RWMutex
	successCnt int
	failureCnt int
	lastCheck  time.Time
	stats      struct {
		totalChecks   int64
		successChecks int64
		failedChecks  int64
		avgRespTime   time.Duration
	}
	stopChan chan struct{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(config *HealthCheckConfig) *HealthChecker {
	if config == nil {
		config = DefaultHealthCheckConfig()
	}
	return &HealthChecker{
		config:   config,
		status:   StatusUnknown,
		stopChan: make(chan struct{}),
	}
}

// Start begins health checking
func (h *HealthChecker) Start(target string) {
	ticker := time.NewTicker(h.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopChan:
			return
		case <-ticker.C:
			h.performCheck(target)
		}
	}
}

// Stop stops health checking
func (h *HealthChecker) Stop() {
	close(h.stopChan)
}

// GetStatus returns the current health status
func (h *HealthChecker) GetStatus() HealthStatus {
	h.statusMu.RLock()
	defer h.statusMu.RUnlock()
	return h.status
}

// performCheck executes a single health check
func (h *HealthChecker) performCheck(target string) {
	ctx, cancel := context.WithTimeout(context.Background(), h.config.Timeout)
	defer cancel()

	startTime := time.Now()
	success := false

	for retry := 0; retry <= h.config.MaxRetries; retry++ {
		if err := h.check(ctx, target); err == nil {
			success = true
			break
		}
		time.Sleep(time.Millisecond * 100) // Brief pause between retries
	}

	respTime := time.Since(startTime)
	h.updateStats(success, respTime)
	h.updateStatus(success)
}

// check performs the actual health check
func (h *HealthChecker) check(ctx context.Context, target string) error {
	// Create a dialer with timeout
	dialer := &net.Dialer{
		Timeout:   h.config.Timeout,
		KeepAlive: 30 * time.Second,
	}

	// Try to establish connection
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	return nil
}

// updateStats updates health check statistics
func (h *HealthChecker) updateStats(success bool, respTime time.Duration) {
	h.stats.totalChecks++
	if success {
		h.stats.successChecks++
	} else {
		h.stats.failedChecks++
	}

	// Update moving average response time
	if h.stats.totalChecks == 1 {
		h.stats.avgRespTime = respTime
	} else {
		h.stats.avgRespTime = (h.stats.avgRespTime + respTime) / 2
	}
}

// updateStatus updates the health status based on check results
func (h *HealthChecker) updateStatus(success bool) {
	h.statusMu.Lock()
	defer h.statusMu.Unlock()

	if success {
		h.successCnt++
		h.failureCnt = 0
		if h.successCnt >= h.config.RiseCount {
			h.status = StatusHealthy
		}
	} else {
		h.failureCnt++
		h.successCnt = 0
		if h.failureCnt >= h.config.FallCount {
			h.status = StatusUnhealthy
		} else if h.status == StatusHealthy {
			h.status = StatusDegraded
		}
	}

	h.lastCheck = time.Now()
}

// GetHealthStats returns current health check statistics
func (h *HealthChecker) GetHealthStats() map[string]interface{} {
	h.statusMu.RLock()
	defer h.statusMu.RUnlock()

	successRate := float64(0)
	if h.stats.totalChecks > 0 {
		successRate = float64(h.stats.successChecks) / float64(h.stats.totalChecks)
	}

	return map[string]interface{}{
		"status":           h.status,
		"lastCheck":        h.lastCheck,
		"totalChecks":      h.stats.totalChecks,
		"successRate":      successRate,
		"avgResponseTime":  h.stats.avgRespTime,
		"successiveChecks": h.successCnt,
		"successiveErrors": h.failureCnt,
	}
}