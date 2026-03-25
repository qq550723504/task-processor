package store

import shein "task-processor/internal/shein"

type SiteInfoHandler struct{}

func NewSiteInfoHandler() *SiteInfoHandler {
	return &SiteInfoHandler{}
}

func (h *SiteInfoHandler) Name() string {
	return "site_info"
}

func (h *SiteInfoHandler) Handle(ctx *shein.TaskContext) error {
	siteList := GetSiteListByRegion(ctx.Task.Region)
	ctx.SetSiteList(siteList)
	return nil
}
