package listingkit

import (
	"context"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

func (s *service) rememberSheinManualResolution(task *Task, req *ApplyRevisionRequest) {
	if s == nil || task == nil || task.Result == nil || task.Result.Shein == nil || req == nil || req.Shein == nil {
		return
	}
	buildReq := buildSheinPublishRequest(task.Request)
	pkg := task.Result.Shein
	canonical := task.Result.CanonicalProduct

	if req.Shein.CategoryResolution != nil {
		if cache, ok := s.sheinCategoryResolver.(sheinpub.CategoryResolutionCache); ok {
			cache.RememberCategoryResolution(buildReq, canonical, pkg, pkg.CategoryResolution)
		}
	}
	if req.Shein.AttributeResolution != nil {
		if cache, ok := s.sheinAttributeResolver.(sheinpub.AttributeResolutionCache); ok {
			cache.RememberAttributeResolution(buildReq, canonical, pkg, pkg.AttributeResolution)
		}
	}
	if req.Shein.SaleAttributeResolution != nil {
		if cache, ok := s.sheinSaleAttributeResolver.(sheinpub.SaleAttributeResolutionCache); ok {
			cache.RememberSaleAttributeResolution(buildReq, canonical, pkg, pkg.SaleAttributeResolution)
		}
	}
}

func (s *service) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*SheinResolutionCacheClearResult, error) {
	if s == nil {
		return nil, ErrTaskNotFound
	}
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil || task.Result.Shein == nil {
		return nil, ErrTaskResultUnavailable
	}

	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "all"
	}
	if kind != "all" && kind != sheinpub.ResolutionCacheKindCategory && kind != sheinpub.ResolutionCacheKindAttribute && kind != sheinpub.ResolutionCacheKindSaleAttribute {
		return nil, ErrInvalidSheinResolutionCacheKind
	}

	buildReq := buildSheinPublishRequest(task.Request)
	pkg := task.Result.Shein
	canonical := task.Result.CanonicalProduct
	deletedKinds := make([]string, 0, 3)

	if kind == "all" || kind == sheinpub.ResolutionCacheKindCategory {
		if cache, ok := s.sheinCategoryResolver.(sheinpub.CategoryResolutionCache); ok {
			if err := cache.ClearCategoryResolution(buildReq, canonical, pkg); err != nil {
				return nil, err
			}
			deletedKinds = append(deletedKinds, sheinpub.ResolutionCacheKindCategory)
		}
	}
	if kind == "all" || kind == sheinpub.ResolutionCacheKindAttribute {
		if cache, ok := s.sheinAttributeResolver.(sheinpub.AttributeResolutionCache); ok {
			if err := cache.ClearAttributeResolution(buildReq, canonical, pkg); err != nil {
				return nil, err
			}
			deletedKinds = append(deletedKinds, sheinpub.ResolutionCacheKindAttribute)
		}
	}
	if kind == "all" || kind == sheinpub.ResolutionCacheKindSaleAttribute {
		if cache, ok := s.sheinSaleAttributeResolver.(sheinpub.SaleAttributeResolutionCache); ok {
			if err := cache.ClearSaleAttributeResolution(buildReq, canonical, pkg); err != nil {
				return nil, err
			}
			deletedKinds = append(deletedKinds, sheinpub.ResolutionCacheKindSaleAttribute)
		}
	}

	return &SheinResolutionCacheClearResult{
		TaskID:       taskID,
		Kind:         kind,
		DeletedKinds: deletedKinds,
	}, nil
}
