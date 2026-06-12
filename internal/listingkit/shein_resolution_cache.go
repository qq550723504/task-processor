package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

func (s *service) rememberSheinSubmittedResolution(task *Task, action string) {
	if s == nil || task == nil || task.Result == nil || task.Result.Shein == nil || strings.TrimSpace(action) != "publish" {
		return
	}
	s.rememberSheinCategoryResolution(task)
	s.rememberSheinAttributeResolution(task)
	s.rememberSheinSaleAttributeResolution(task)
	s.rememberSheinSubmittedPricing(task, action)
}

func (s *service) rememberSheinCategoryResolution(task *Task) {
	if s == nil || task == nil || task.Result == nil || task.Result.Shein == nil {
		return
	}
	if cache, ok := resolveSheinCategoryResolver(s).(sheinpub.CategoryResolutionCache); ok {
		cache.RememberCategoryResolution(buildSheinPublishRequest(task.Request), task.Result.CanonicalProduct, task.Result.Shein, task.Result.Shein.CategoryResolution)
	}
}

func (s *service) rememberSheinAttributeResolution(task *Task) {
	if s == nil || task == nil || task.Result == nil || task.Result.Shein == nil {
		return
	}
	if cache, ok := resolveSheinAttributeResolver(s).(sheinpub.AttributeResolutionCache); ok {
		cache.RememberAttributeResolution(buildSheinPublishRequest(task.Request), task.Result.CanonicalProduct, task.Result.Shein, task.Result.Shein.AttributeResolution)
	}
}

func (s *service) rememberSheinSaleAttributeResolution(task *Task) {
	if s == nil || task == nil || task.Result == nil || task.Result.Shein == nil {
		return
	}
	if cache, ok := resolveSheinSaleAttributeResolver(s).(sheinpub.SaleAttributeResolutionCache); ok {
		cache.RememberSaleAttributeResolution(buildSheinPublishRequest(task.Request), task.Result.CanonicalProduct, task.Result.Shein, task.Result.Shein.SaleAttributeResolution)
	}
}
