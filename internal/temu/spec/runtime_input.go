package spec

import (
	"fmt"

	temuapi "task-processor/internal/temu/api"
	temucontext "task-processor/internal/temu/context"
)

type ResolveSpecRuntimeInput struct {
	APIClient temuapi.APIClientInterface
	GoodsID   string
}

func BuildResolveSpecRuntimeInput(temuCtx *temucontext.TemuTaskContext) (*ResolveSpecRuntimeInput, error) {
	if temuCtx == nil {
		return nil, fmt.Errorf("temu context is nil")
	}
	if temuCtx.APIClient == nil {
		return nil, fmt.Errorf("api client is not initialized")
	}
	if temuCtx.TemuProduct == nil || temuCtx.TemuProduct.GoodsBasic.GoodsID == "" {
		return nil, fmt.Errorf("goods_id is not set")
	}

	return &ResolveSpecRuntimeInput{
		APIClient: temuCtx.APIClient,
		GoodsID:   temuCtx.TemuProduct.GoodsBasic.GoodsID,
	}, nil
}
