// Package utils 提供工具方法
package utils

import "strings"

// ParsePlatformList 解析平台列表字符串
// 输入: "amazon,temu,shein" 或 "amazon, temu, shein"
// 输出: ["amazon", "temu", "shein"]
func ParsePlatformList(platformsStr string) []string {
	if platformsStr == "" {
		return []string{}
	}

	platforms := strings.Split(platformsStr, ",")
	result := make([]string, 0, len(platforms))

	for _, platform := range platforms {
		trimmed := strings.TrimSpace(platform)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// ContainsPlatform 检查平台列表是否包含指定平台（不区分大小写）
func ContainsPlatform(platforms []string, platform string) bool {
	for _, p := range platforms {
		if strings.EqualFold(p, platform) {
			return true
		}
	}
	return false
}
