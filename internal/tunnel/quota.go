package tunnel

import "context"

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

// QuotaChecker is the minimal interface used by tunnel servers to enforce quotas
type QuotaChecker interface {
	CheckUser(ctx context.Context, userID uint32) (QuotaResult, error)
}
