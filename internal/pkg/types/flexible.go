// Package types 提供通用的灵活类型定义
package types

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// FlexibleID 灵活的ID类型，可以处理字符串或整数
type FlexibleID int

// UnmarshalJSON 自定义JSON反序列化，支持字符串和整数
func (f *FlexibleID) UnmarshalJSON(data []byte) error {
	// 尝试作为整数解析
	var intVal int
	if err := json.Unmarshal(data, &intVal); err == nil {
		*f = FlexibleID(intVal)
		return nil
	}

	// 尝试作为字符串解析
	var strVal string
	if err := json.Unmarshal(data, &strVal); err == nil {
		if strVal == "" {
			*f = FlexibleID(0)
			return nil
		}
		if intVal, err := strconv.Atoi(strVal); err == nil {
			*f = FlexibleID(intVal)
			return nil
		}
		// 如果字符串无法转换为整数，设为0
		*f = FlexibleID(0)
		return nil
	}

	// 如果都失败了，设为0
	*f = FlexibleID(0)
	return nil
}

// Int 返回整数值
func (f FlexibleID) Int() int {
	return int(f)
}

// FlexibleString 可以接受字符串或数字的灵活字符串类型
type FlexibleString string

// UnmarshalJSON 自定义JSON解析，支持字符串和数字
func (fs *FlexibleString) UnmarshalJSON(data []byte) error {
	// 尝试解析为字符串
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*fs = FlexibleString(str)
		return nil
	}

	// 尝试解析为数字
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		*fs = FlexibleString(fmt.Sprintf("%.2f", num))
		return nil
	}

	// 如果都失败，返回错误
	return fmt.Errorf("cannot unmarshal %s into FlexibleString", string(data))
}

// String 返回字符串值
func (fs FlexibleString) String() string {
	return string(fs)
}

// FlexibleTime 灵活的时间类型，可以处理多种时间格式
type FlexibleTime struct {
	time.Time
}

// UnmarshalJSON 自定义 JSON 反序列化，支持多种时间格式
func (ft *FlexibleTime) UnmarshalJSON(data []byte) error {
	// 如果是 null，返回零值
	if string(data) == "null" {
		return nil
	}

	// 尝试作为字符串解析
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str == "" {
			return nil
		}

		// 尝试多种时间格式
		formats := []string{
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05Z",
			"2006-01-02T15:04:05.000Z",
			"2006-01-02T15:04:05+08:00",
			"2006-01-02T15:04:05.000+08:00",
			time.RFC3339,
			time.RFC3339Nano,
		}

		for _, format := range formats {
			if t, err := time.Parse(format, str); err == nil {
				ft.Time = t
				return nil
			}
		}

		return fmt.Errorf("无法解析时间格式: %s", str)
	}

	// 尝试作为时间戳解析（自动检测秒级或毫秒级）
	var timestamp int64
	if err := json.Unmarshal(data, &timestamp); err == nil {
		// 如果时间戳大于 10000000000（表示 2286-11-20），则视为毫秒级时间戳
		// 正常的秒级时间戳在可预见的未来不会超过这个值
		if timestamp > 10000000000 {
			// 毫秒级时间戳，转换为秒和纳秒
			ft.Time = time.Unix(timestamp/1000, (timestamp%1000)*1000000)
		} else {
			// 秒级时间戳
			ft.Time = time.Unix(timestamp, 0)
		}
		return nil
	}

	return fmt.Errorf("无法解析时间: %s", string(data))
}

// MarshalJSON 自定义 JSON 序列化
func (ft FlexibleTime) MarshalJSON() ([]byte, error) {
	if ft.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(ft.Time.Format("2006-01-02 15:04:05"))
}

// Format 格式化时间
func (ft FlexibleTime) Format(layout string) string {
	return ft.Time.Format(layout)
}

// IsZero 判断是否为零值
func (ft FlexibleTime) IsZero() bool {
	return ft.Time.IsZero()
}

// ToFlexibleTime 将 *time.Time 转换为 *FlexibleTime
func ToFlexibleTime(t *time.Time) *FlexibleTime {
	if t == nil {
		return nil
	}
	return &FlexibleTime{Time: *t}
}
