// Package utils 提供工具方法
package utils

import "task-processor/internal/pkg/amazon"

// GetDomainMap 获取地区到域名的映射（已废弃，使用 amazon.GetDomainMap）
// Deprecated: 使用 amazon.GetDomainMap() 代替
func GetDomainMap() map[string]string {
	return amazon.GetDomainMap()
}

// GetZipcodeMap 获取地区到默认邮编的映射（已废弃，使用 amazon.GetZipcodeMap）
// Deprecated: 使用 amazon.GetZipcodeMap() 代替
func GetZipcodeMap() map[string]string {
	return amazon.GetZipcodeMap()
}

// GetDefaultDomain 获取默认域名（已废弃，使用 amazon.GetDefaultDomain）
// Deprecated: 使用 amazon.GetDefaultDomain() 代替
func GetDefaultDomain(region string) string {
	return amazon.GetDefaultDomain(region)
}

// GetDefaultZipcode 获取默认邮编（已废弃，使用 amazon.GetDefaultZipcode）
// Deprecated: 使用 amazon.GetDefaultZipcode() 代替
func GetDefaultZipcode(region string) string {
	return amazon.GetDefaultZipcode(region)
}
