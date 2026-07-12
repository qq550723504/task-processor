package store

import (
	"fmt"
	"task-processor/internal/core/logger"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/siteconfig"
)

type SiteInfoHandler struct{}

func NewSiteInfoHandler() *SiteInfoHandler {
	return &SiteInfoHandler{}
}

func (h *SiteInfoHandler) Name() string {
	return "site_info"
}

func (h *SiteInfoHandler) Handle(ctx *shein.TaskContext) error {
	region := ""
	if ctx.Task != nil {
		region = ctx.Task.Region
	}
	var siteList = GetSiteListByRegion(region)
	if ctx.ProductAPI != nil {
		groups, err := ctx.ProductAPI.QuerySiteList()
		if err != nil {
			logger.GetGlobalLogger("shein/store").Warnf("query supplier site list failed: %v; using region fallback", err)
		} else if dynamic := siteconfig.Normalize(groups); len(dynamic) > 0 {
			siteList = dynamic
		} else {
			logger.GetGlobalLogger("shein/store").Warn("supplier site list contains no enabled sites; using region fallback")
		}
	}
	if len(siteList) == 0 {
		return fmt.Errorf("no SHEIN sites available for region %q", region)
	}
	ctx.SetSiteList(siteList)
	return nil
}
