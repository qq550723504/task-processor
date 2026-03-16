package shein

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type JSONMap map[string]any

// Value 实现 driver.Valuer 接口，用于数据库存储
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口，用于数据库读取
func (j *JSONMap) Scan(value any) error {
	if j == nil {
		// 修复nil指针解引用问题
		return fmt.Errorf("cannot scan to nil JSONMap pointer")
	}
	if value == nil {
		*j = make(JSONMap)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into JSONMap", value)
	}

	if len(bytes) == 0 {
		*j = make(JSONMap)
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// MarshalJSON 实现 json.Marshaler 接口
func (j JSONMap) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]any(j))
}

// UnmarshalJSON 实现 json.Unmarshaler 接口
func (j *JSONMap) UnmarshalJSON(data []byte) error {
	if j == nil {
		// 修复nil指针解引用问题
		return fmt.Errorf("cannot unmarshal to nil JSONMap pointer")
	}

	// 先尝试解析为 map[string]any
	var mapData map[string]any
	if err := json.Unmarshal(data, &mapData); err == nil {
		*j = JSONMap(mapData)
		return nil
	}

	// 如果解析为 map 失败，尝试解析为其他类型并包装
	var rawData any
	if err := json.Unmarshal(data, &rawData); err != nil {
		return fmt.Errorf("无法解析JSON数据: %w", err)
	}

	// 根据数据类型进行包装
	switch v := rawData.(type) {
	case []any:
		*j = JSONMap{
			"data":  v,
			"type":  "array",
			"count": len(v),
		}
	default:
		*j = JSONMap{
			"value": v,
			"type":  fmt.Sprintf("%T", v),
		}
	}

	return nil
}

// Get 获取指定键的值
func (j JSONMap) Get(key string) (any, bool) {
	if j == nil {
		return nil, false
	}
	value, exists := j[key]
	return value, exists
}

// GetString 获取字符串类型的值
func (j JSONMap) GetString(key string) (string, bool) {
	if value, exists := j.Get(key); exists {
		if str, ok := value.(string); ok {
			return str, true
		}
	}
	return "", false
}

// GetInt 获取整数类型的值
func (j JSONMap) GetInt(key string) (int, bool) {
	if value, exists := j.Get(key); exists {
		switch v := value.(type) {
		case int:
			return v, true
		case float64:
			return int(v), true
		}
	}
	return 0, false
}

// GetFloat64 获取浮点数类型的值
func (j JSONMap) GetFloat64(key string) (float64, bool) {
	if value, exists := j.Get(key); exists {
		switch v := value.(type) {
		case float64:
			return v, true
		case int:
			return float64(v), true
		}
	}
	return 0, false
}

// GetBool 获取布尔类型的值
func (j JSONMap) GetBool(key string) (bool, bool) {
	if value, exists := j.Get(key); exists {
		if b, ok := value.(bool); ok {
			return b, true
		}
	}
	return false, false
}

// Set 设置键值对
func (j *JSONMap) Set(key string, value any) {
	if j == nil {
		// 修复nil指针解引用问题
		return
	}
	if *j == nil {
		*j = make(JSONMap)
	}
	(*j)[key] = value
}

// Delete 删除指定键
func (j JSONMap) Delete(key string) {
	if j != nil {
		delete(j, key)
	}
}

// Keys 获取所有键
func (j JSONMap) Keys() []string {
	if j == nil {
		return []string{}
	}
	keys := make([]string, 0, len(j))
	for k := range j {
		keys = append(keys, k)
	}
	return keys
}

// IsEmpty 检查是否为空
func (j JSONMap) IsEmpty() bool {
	return len(j) == 0
}

// Clone 创建副本
func (j JSONMap) Clone() JSONMap {
	if j == nil {
		return make(JSONMap)
	}
	clone := make(JSONMap, len(j))
	for k, v := range j {
		clone[k] = v
	}
	return clone
}

// Merge 合并另一个JSONMap
func (j *JSONMap) Merge(other JSONMap) {
	if j == nil || other == nil {
		return
	}
	if *j == nil {
		*j = make(JSONMap)
	}
	for k, v := range other {
		(*j)[k] = v
	}
}

// IsWrappedArray 检查是否为包装的数组数据
func (j JSONMap) IsWrappedArray() bool {
	if j == nil {
		return false
	}
	dataType, exists := j["type"]
	return exists && dataType == "array"
}

// GetWrappedArrayData 获取包装的数组数据
func (j JSONMap) GetWrappedArrayData() ([]any, bool) {
	if !j.IsWrappedArray() {
		return nil, false
	}
	if data, exists := j["data"]; exists {
		if arrayData, ok := data.([]any); ok {
			return arrayData, true
		}
	}
	return nil, false
}

// IsWrappedValue 检查是否为包装的基础类型值
func (j JSONMap) IsWrappedValue() bool {
	if j == nil {
		return false
	}
	dataType, exists := j["type"]
	return exists && dataType != "array" && dataType != "object"
}

// GetWrappedValue 获取包装的基础类型值
func (j JSONMap) GetWrappedValue() (any, bool) {
	if !j.IsWrappedValue() {
		return nil, false
	}
	if value, exists := j["value"]; exists {
		return value, true
	}
	return nil, false
}
