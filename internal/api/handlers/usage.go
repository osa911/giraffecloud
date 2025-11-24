package handlers

import (
	"strconv"
	"time"

	"github.com/osa911/giraffecloud/internal/api/constants"
	"github.com/osa911/giraffecloud/internal/api/dto/common"
	"github.com/osa911/giraffecloud/internal/db/ent"
	"github.com/osa911/giraffecloud/internal/db/ent/usage"
	"github.com/osa911/giraffecloud/internal/service"
	"github.com/osa911/giraffecloud/internal/utils"

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
	// Get user from context (set by auth middleware)
	userModel, exists := c.Get(constants.ContextKeyUser)
	if !exists {
		utils.HandleAPIError(c, nil, common.ErrCodeUnauthorized, "User not found in context")
		return
	}

	currentUser, ok := userModel.(*ent.User)
	if !ok {
		utils.HandleAPIError(c, nil, common.ErrCodeInternalServer, "Invalid user type in context")
		return
	}

	userID := currentUser.ID

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

// GetDailyHistory returns daily usage for the last N days
func (h *UsageHandler) GetDailyHistory(c *gin.Context) {
	// Get user from context (set by auth middleware)
	userModel, exists := c.Get(constants.ContextKeyUser)
	if !exists {
		utils.HandleAPIError(c, nil, common.ErrCodeUnauthorized, "User not found in context")
		return
	}

	currentUser, ok := userModel.(*ent.User)
	if !ok {
		utils.HandleAPIError(c, nil, common.ErrCodeInternalServer, "Invalid user type in context")
		return
	}

	userID := currentUser.ID

	// Get days parameter (default to 30)
	days := 30
	if daysParam := c.Query("days"); daysParam != "" {
		if parsedDays, err := strconv.Atoi(daysParam); err == nil && parsedDays > 0 {
			days = parsedDays
		}
	}

	// Calculate start date (N days ago)
	now := time.Now().UTC()
	startDate := now.AddDate(0, 0, -days).Truncate(24 * time.Hour)

	// Fetch usage data for the last N days
	usageRows, err := h.db.Usage.Query().
		Where(
			usage.UserIDEQ(uint32(userID)),
			usage.PeriodStartGTE(startDate),
		).
		Order(ent.Asc(usage.FieldPeriodStart)).
		All(c)

	if err != nil {
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to fetch usage history")
		return
	}

	// Group by day and aggregate
	dailyUsage := make(map[string]map[string]int64)

	for _, row := range usageRows {
		dayKey := row.PeriodStart.Format("2006-01-02")
		if dailyUsage[dayKey] == nil {
			dailyUsage[dayKey] = map[string]int64{
				"bytes_in":  0,
				"bytes_out": 0,
				"requests":  0,
			}
		}
		dailyUsage[dayKey]["bytes_in"] += row.BytesIn
		dailyUsage[dayKey]["bytes_out"] += row.BytesOut
		dailyUsage[dayKey]["requests"] += row.Requests
	}

	// Build response array with all days (including zeros for days with no data)
	var result []gin.H
	currentDate := startDate
	for currentDate.Before(now) || currentDate.Equal(now.Truncate(24*time.Hour)) {
		dayKey := currentDate.Format("2006-01-02")
		data := dailyUsage[dayKey]

		result = append(result, gin.H{
			"date":      dayKey,
			"bytes_in":  data["bytes_in"],
			"bytes_out": data["bytes_out"],
			"requests":  data["requests"],
			"total":     data["bytes_in"] + data["bytes_out"],
		})

		currentDate = currentDate.AddDate(0, 0, 1)
	}

	utils.HandleSuccess(c, gin.H{
		"history": result,
		"days":    days,
	})
}
