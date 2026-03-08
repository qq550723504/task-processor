// Package loaders 提供配置加载功能
package loaders

import (
	"time"

	"github.com/spf13/viper"
)

// getDuration 获取时长配置（秒转换为 time.Duration）
func getDuration(key string, defaultSeconds int) time.Duration {
	seconds := viper.GetInt(key)
	if seconds == 0 {
		seconds = defaultSeconds
	}
	return time.Duration(seconds) * time.Second
}

// getInt64Slice 获取int64切片
func getInt64Slice(key string) []int64 {
	intSlice := viper.GetIntSlice(key)
	result := make([]int64, len(intSlice))
	for i, v := range intSlice {
		result[i] = int64(v)
	}
	return result
}

// getStringFromMap 从 map 中获取字符串值
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getIntFromMap 从 map 中获取整数值
func getIntFromMap(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}
