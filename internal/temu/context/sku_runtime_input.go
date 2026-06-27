package context

import (
	"fmt"

	"task-processor/internal/listingadmin"
)

type SKUBuildRuntime struct {
	TenantID   int64
	StoreID    int64
	StoreInfo  *listingadmin.StoreRespDTO
	AsinSkuMap map[string]string
}

func BuildSKUBuildRuntime(temuCtx *TemuTaskContext) (*SKUBuildRuntime, error) {
	if temuCtx == nil {
		return nil, fmt.Errorf("temu context is nil")
	}

	tenantID := int64(1)
	storeID := int64(1)
	if task := temuCtx.GetTask(); task != nil {
		if task.TenantID != 0 {
			tenantID = task.TenantID
		}
		if task.StoreID != 0 {
			storeID = task.StoreID
		}
	}

	return &SKUBuildRuntime{
		TenantID:   tenantID,
		StoreID:    storeID,
		StoreInfo:  temuCtx.StoreInfo,
		AsinSkuMap: temuCtx.AsinSkuMap,
	}, nil
}

func (r *SKUBuildRuntime) SaveAsinSkuMapping(outSkuSN, asin string) map[string]string {
	if r.AsinSkuMap == nil {
		r.AsinSkuMap = make(map[string]string)
	}
	r.AsinSkuMap[outSkuSN] = asin
	return r.AsinSkuMap
}
