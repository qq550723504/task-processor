// Package product 提供Amazon域名解析功能
package product

import (
	pkgamazon "task-processor/internal/pkg/amazon"
)

// DomainResolver Amazon域名解析器（包装共享的实现）
type DomainResolver struct {
	resolver *pkgamazon.DomainResolver
}

// NewDomainResolver 创建域名解析器
func NewDomainResolver() *DomainResolver {
	return &DomainResolver{
		resolver: pkgamazon.NewDomainResolver(),
	}
}

// GetAmazonDomainByRegion 根据地区获取Amazon域名
func (r *DomainResolver) GetAmazonDomainByRegion(region string) string {
	return r.resolver.GetAmazonDomainByRegion(region)
}

// BuildAmazonProductURL 构建Amazon产品URL(统一入口)
func (r *DomainResolver) BuildAmazonProductURL(region, asin string) string {
	return r.resolver.BuildAmazonProductURL(region, asin)
}

// GetZipcodeByRegion 根据地区获取邮编
func (r *DomainResolver) GetZipcodeByRegion(region string) string {
	return r.resolver.GetZipcodeByRegion(region)
}
