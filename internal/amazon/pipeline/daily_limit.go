package pipeline

import (
	"context"
	"fmt"

	"task-processor/internal/amazon/model"
	"task-processor/internal/pkg/timex"
)

// DailyLimitHandler 在 Amazon 发布前原子检查并预占每日额度。
type DailyLimitHandler struct {
	*BaseHandler
	services *model.Services
}

func NewDailyLimitHandler(services *model.Services) *DailyLimitHandler {
	return &DailyLimitHandler{
		BaseHandler: NewBaseHandler("检查每日上架限额", amazonTaskStageCheckDailyLimit),
		services:    services,
	}
}

func (h *DailyLimitHandler) Handle(ctx context.Context, taskContext *model.TaskContext) error {
	if taskContext == nil || taskContext.StoreInfo == nil {
		return nil
	}
	if h.services == nil || h.services.MemoryManager == nil {
		return nil
	}
	if taskContext.StoreInfo.DailyLimit == nil || *taskContext.StoreInfo.DailyLimit <= 0 {
		return nil
	}

	tenantID, err := h.GetRequiredInt64(taskContext.Data, "tenantId")
	if err != nil {
		return err
	}
	storeID, err := h.GetRequiredInt64(taskContext.Data, "storeId")
	if err != nil {
		return err
	}

	dailyLimit := *taskContext.StoreInfo.DailyLimit
	currentDate := timex.NowDate()
	increment := h.calculateIncrement(taskContext)

	reservation, err := h.services.MemoryManager.DailyCountManager.TryReserveQuota(
		tenantID,
		storeID,
		currentDate,
		increment,
		int64(dailyLimit),
	)
	if err != nil {
		return err
	}

	currentCount := reservation.NewCount
	if reservation.Allowed {
		currentCount = reservation.NewCount - increment
	}

	if !reservation.Allowed {
		h.services.MemoryManager.ShopPauseManager.PauseShopUntilEndOfDay(
			tenantID,
			storeID,
			fmt.Sprintf("达到每日上架限额(%d/%d)", currentCount, dailyLimit),
		)
		return newNonRetryableStageError(
			amazonTaskStageCheckDailyLimit,
			amazonTaskReasonDailyLimitReached,
			"店铺已达到每日上架限额(%d/%d)，已暂停上架到当日结束",
			currentCount,
			dailyLimit,
		)
	}

	taskContext.SetDailyQuotaReservation(currentDate, increment)
	return nil
}

func (h *DailyLimitHandler) calculateIncrement(taskContext *model.TaskContext) int64 {
	limitType := taskContext.StoreInfo.DailyLimitType
	if limitType == "" {
		limitType = "SPU"
	}

	switch limitType {
	case "SKU", "SKC":
		if count, exists := taskContext.GetResult("variant_children_count"); exists {
			switch value := count.(type) {
			case int:
				if value > 0 {
					return int64(value)
				}
			case int64:
				if value > 0 {
					return value
				}
			case float64:
				if value > 0 {
					return int64(value)
				}
			}
		}
	}

	return 1
}
