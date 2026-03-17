package pricing

import (
	"fmt"
	"time"
)

const (
	// SheinTimeFormat SHEIN平台使用的时间格式
	SheinTimeFormat = "2006-01-02 15:04:05"
)

// TimeHelper 时间处理工具
type TimeHelper struct{}

// NewTimeHelper 创建时间处理工具实例
func NewTimeHelper() *TimeHelper {
	return &TimeHelper{}
}

// FormatTime 格式化时间为SHEIN平台格式
func (h *TimeHelper) FormatTime(t time.Time) string {
	return t.Format(SheinTimeFormat)
}

// ParseTime 解析SHEIN平台时间格式
func (h *TimeHelper) ParseTime(timeStr string) (time.Time, error) {
	return time.Parse(SheinTimeFormat, timeStr)
}

// GetDefaultTimeRange 获取默认的时间范围（最近三个月）
func (h *TimeHelper) GetDefaultTimeRange() (startTime, endTime string) {
	now := time.Now()
	end := now.Add(24 * time.Hour)
	start := end.AddDate(0, -3, 0)
	return h.FormatTime(start), h.FormatTime(end)
}

// ValidateTimeRange 验证时间范围是否有效
func (h *TimeHelper) ValidateTimeRange(startTime, endTime string) error {
	if startTime == "" || endTime == "" {
		return nil
	}
	start, err := h.ParseTime(startTime)
	if err != nil {
		return err
	}
	end, err := h.ParseTime(endTime)
	if err != nil {
		return err
	}
	if start.After(end) {
		return fmt.Errorf("开始时间不能晚于结束时间")
	}
	if end.Sub(start) > 90*24*time.Hour {
		return fmt.Errorf("时间周期不能超过三个月")
	}
	return nil
}

// GetMaxAllowedTimeRange 获取最大允许的时间范围（三个月）
func (h *TimeHelper) GetMaxAllowedTimeRange(endTime time.Time) time.Time {
	return endTime.AddDate(0, -3, 0)
}

// AdjustTimeRangeToLimit 调整时间范围以符合API限制
func (h *TimeHelper) AdjustTimeRangeToLimit(startTime, endTime string) (adjustedStart, adjustedEnd string, err error) {
	if startTime == "" || endTime == "" {
		defaultStart, defaultEnd := h.GetDefaultTimeRange()
		return defaultStart, defaultEnd, nil
	}
	start, err := h.ParseTime(startTime)
	if err != nil {
		return "", "", err
	}
	end, err := h.ParseTime(endTime)
	if err != nil {
		return "", "", err
	}
	if end.Sub(start) > 90*24*time.Hour {
		start = end.Add(-90 * 24 * time.Hour)
	}
	return h.FormatTime(start), h.FormatTime(end), nil
}
