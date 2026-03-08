// Package config 提供配置管理功能
package config

import (
	"task-processor/internal/core/config/types"

	"github.com/spf13/viper"
)

// getInt64Slice 从配置中获取 int64 切片
func getInt64Slice(key string) []int64 {
	values := viper.GetIntSlice(key)
	result := make([]int64, len(values))
	for i, v := range values {
		result[i] = int64(v)
	}
	return result
}

// loadSPAPIConfig 加载 Amazon SP-API 配置
// 注意: 此函数在 builder.go 中被间接调用,不要删除
func loadSPAPIConfig() types.SPAPIConfig {
	config := types.SPAPIConfig{
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
	}

	// 加载市场配置
	config.Marketplaces = make(map[string]types.MarketplaceConfig)

	// 预定义的市场配置
	markets := []string{"us", "ca", "mx", "br"}
	for _, market := range markets {
		marketKey := "amazon.spapi.markets." + market
		if viper.IsSet(marketKey) {
			config.Marketplaces[market] = types.MarketplaceConfig{
				Name:          viper.GetString(marketKey + ".name"),
				MarketplaceID: viper.GetString(marketKey + ".marketplaceID"),
				Currency:      viper.GetString(marketKey + ".currency"),
				Enabled:       viper.GetBool(marketKey + ".enabled"),
			}
		}
	}

	return config
}
