package handlers

import (
	"time"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/db/ent/usage"
	"giraffecloud/internal/service"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
)

type UsageHandler struct {
	db    *ent.Client
	quota service.QuotaService
}

func NewUsageHandler(db *ent.Client, quota service.QuotaService) *UsageHandler {
	return &UsageHandler{db: db, quota: quota}
}

// GetSummary returns current cycle usage summary for the authenticated user
func (h *UsageHandler) GetSummary(c *gin.Context) {
	userID := uint32(c.GetUint(constants.ContextKeyUserID))
	if userID == 0 {
		utils.HandleAPIError(c, nil, common.ErrCodeUnauthorized, "Unauthorized")
		return
	}

	// Today summary (daily aggregation)
	dayStart := time.Now().UTC().Truncate(24 * time.Hour)
	dayRows, err := h.db.Usage.Query().
		Where(
			usage.PeriodStartEQ(dayStart),
			usage.UserIDEQ(uint32(userID)),
		).All(c)
	if err != nil {
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to fetch usage")
		return
	}
	var dayIn, dayOut, dayReq int64
	for _, r := range dayRows {
		dayIn += r.BytesIn
		dayOut += r.BytesOut
		dayReq += r.Requests
	}

	// Monthly quota summary
	quotaRes, _ := h.quota.CheckUser(c, uint32(userID))

	utils.HandleSuccess(c, gin.H{
		"day": gin.H{
			"period_start": dayStart,
			"bytes_in":     dayIn,
			"bytes_out":    dayOut,
			"requests":     dayReq,
		},
		"month": gin.H{
			"used_bytes":  quotaRes.UsedBytes,
			"limit_bytes": quotaRes.LimitBytes,
			"decision":    string(quotaRes.Decision),
		},
	})
}
