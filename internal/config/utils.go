// Package config 提供配置管理功能
package config

import (
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// getInt64Slice 从viper获取int64切片的辅助函数
func getInt64Slice(key string) []int64 {
	if ifaceSlice := viper.Get(key); ifaceSlice != nil {
		switch v := ifaceSlice.(type) {
		case []any:
			result := make([]int64, len(v))
			for i, val := range v {
				switch val := val.(type) {
				case int64:
					result[i] = val
				case int:
					result[i] = int64(val)
				case float64:
					result[i] = int64(val)
				case string:
					if intVal, err := strconv.ParseInt(val, 10, 64); err == nil {
						result[i] = intVal
					}
				}
			}
			return result
		case []int64:
			return v
		case []int:
			result := make([]int64, len(v))
			for i, val := range v {
				result[i] = int64(val)
			}
			return result
		case string:
			if v != "" {
				parts := strings.Split(v, ",")
				result := make([]int64, 0, len(parts))
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if part != "" {
						if intVal, err := strconv.ParseInt(part, 10, 64); err == nil {
							result = append(result, intVal)
						}
					}
				}
				return result
			}
		}
	}
	return []int64{}
}
