package store

import (
	"task-processor/internal/platforms/shein"
	"task-processor/internal/platforms/shein/model"
)

// SiteInfoHandler 站点信息处理器
type SiteInfoHandler struct {
}

// NewSiteInfoHandler 创建新的站点信息处理器
func NewSiteInfoHandler() *SiteInfoHandler {
	return &SiteInfoHandler{}
}

// Name 返回步骤名称
func (h *SiteInfoHandler) Name() string {
	return "设置站点信息"
}

// Handle 执行步骤处理
func (h *SiteInfoHandler) Handle(ctx *model.TaskContext) error {
	// 根据Task中的区域信息设置站点信息
	siteList := shein.GetSiteListByRegion(ctx.Task.Region)

	// 将站点信息存储到上下文中
	ctx.SiteList = siteList

	// 如果ProductData已存在，也更新其中的站点信息
	if ctx.ProductData != nil {
		ctx.ProductData.SiteList = siteList
	}

	return nil
}
