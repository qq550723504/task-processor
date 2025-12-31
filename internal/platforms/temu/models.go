package temu

import (
	"fmt"
	"time"
)

// ReplicateButtonConfig 复制按钮配置
type ReplicateButtonConfig struct {
	AllowOperate bool `json:"allow_operate"`
	AllowShow    bool `json:"allow_show"`
}

// ParseTime 解析 TEMU 时间格式（毫秒时间戳）
func ParseTime(timeStr string) (*time.Time, error) {
	if timeStr == "" {
		return nil, nil
	}
	// TEMU 时间格式: "1742177867237" (毫秒时间戳)
	var timestamp int64
	if _, err := fmt.Sscanf(timeStr, "%d", &timestamp); err != nil {
		return nil, err
	}
	t := time.Unix(timestamp/1000, (timestamp%1000)*1000000)
	return &t, nil
}
