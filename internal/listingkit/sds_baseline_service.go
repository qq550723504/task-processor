package listingkit

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/catalog/canonical"
)

type sdsBaselineService struct {
	repo Repository
}

func (s *service) sdsBaselineOrDefault() *sdsBaselineService {
	if s == nil {
		return &sdsBaselineService{}
	}
	return &sdsBaselineService{repo: s.repo}
}

func (b *sdsBaselineService) GetReadyBaseline(ctx context.Context, task *Task) (*canonical.Product, bool, error) {
	if b == nil || task == nil || task.Request == nil || task.Request.Options == nil {
		return nil, false, nil
	}
	cacheRepo, ok := b.repo.(SDSBaselineCacheRepository)
	if !ok {
		return nil, false, nil
	}
	sdsOptions := task.Request.Options.SDS
	tenantID := strings.TrimSpace(task.Request.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(task.TenantID)
	}
	baselineKey := sdsBaselineKey(tenantID, sdsOptions)
	if baselineKey == "" {
		return nil, false, nil
	}
	entry, err := cacheRepo.GetSDSBaselineCache(ctx, tenantID, baselineKey)
	if err != nil {
		return nil, false, err
	}
	if entry == nil || !strings.EqualFold(strings.TrimSpace(entry.Status), "ready") {
		return nil, false, nil
	}
	if entry.CanonicalProductBase == nil {
		return nil, false, fmt.Errorf("sds baseline %q is ready but missing canonical payload", baselineKey)
	}
	product, err := entry.CanonicalProduct()
	if err != nil {
		return nil, false, err
	}
	if product == nil {
		return nil, false, fmt.Errorf("sds baseline %q resolved to empty canonical product", baselineKey)
	}
	return product, true, nil
}
