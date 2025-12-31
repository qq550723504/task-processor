// Package utils 提供工具方法
package utils

import "fmt"

// URLBuilder URL构建器
type URLBuilder struct{}

// NewURLBuilder 创建URL构建器
func NewURLBuilder() *URLBuilder {
	return &URLBuilder{}
}

// BuildDefaultURL 根据地区构建默认URL
func (u *URLBuilder) BuildDefaultURL(region string) string {
	domain := GetDefaultDomain(region)
	languageParam := "en_US"
	return fmt.Sprintf("https://www.%s/dp/B0DF49ML4P?language=%s", domain, languageParam)
}
