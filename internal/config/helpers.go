// Package config 提供配置辅助函数
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

// loadSPAPIConfig 加载SP-API配置，包括markets
func loadSPAPIConfig() SPAPIConfig {
	config := SPAPIConfig{
		Enabled:                viper.GetBool("amazon.spapi.enabled"),
		Region:                 viper.GetString("amazon.spapi.region"),
		DefaultMarketplace:     viper.GetString("amazon.spapi.defaultMarketplace"),
		ClientID:               viper.GetString("amazon.spapi.clientID"),
		ClientSecret:           viper.GetString("amazon.spapi.clientSecret"),
		RefreshToken:           viper.GetString("amazon.spapi.refreshToken"),
		AWSAccessKeyID:         viper.GetString("amazon.spapi.awsAccessKeyID"),
		AWSSecretKey:           viper.GetString("amazon.spapi.awsSecretKey"),
		DefaultFulfillmentType: viper.GetString("amazon.spapi.defaultFulfillmentType"),
		DefaultCondition:       viper.GetString("amazon.spapi.defaultCondition"),
		// 向后兼容字段
		MarketplaceID: viper.GetString("amazon.spapi.marketplaceID"),
		SellerID:      viper.GetString("amazon.spapi.sellerID"),
	}

	// 加载markets配置
	marketsConfig := viper.GetStringMap("amazon.spapi.markets")
	if marketsConfig != nil && len(marketsConfig) > 0 {
		config.Marketplaces = make(map[string]MarketplaceConfig)

		for marketCode, marketData := range marketsConfig {
			if marketMap, ok := marketData.(map[string]interface{}); ok {
				marketplace := MarketplaceConfig{
					Name:          getStringFromMap(marketMap, "name"),
					MarketplaceID: getStringFromMap(marketMap, "marketplaceid"),
					Currency:      getStringFromMap(marketMap, "currency"),
					SellerID:      getStringFromMap(marketMap, "sellerid"),
					Enabled:       getBoolFromMap(marketMap, "enabled"),
				}
				config.Marketplaces[marketCode] = marketplace
			}
		}
	}

	return config
}

// getStringFromMap 从map中安全获取字符串值
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getBoolFromMap 从map中安全获取布尔值
func getBoolFromMap(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}
