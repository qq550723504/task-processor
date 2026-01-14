// Package types 提供通用的灵活类型定义
package types

import (
	"encoding/json"
	"fmt"
	"strconv"
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
