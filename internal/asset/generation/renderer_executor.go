package generation

import (
	"context"
	"strings"

	"task-processor/internal/asset"
)

func (s *service) executeRendererBackedTask(ctx context.Context, req DispatchRequest, task Task) (asset.AssetRecord, bool) {
	if s.deferredRenderer == nil {
		return asset.AssetRecord{}, false
	}
	base, ok := preferredDeferredBaseRecord(req.Inventory, task)
	if !ok {
		return asset.AssetRecord{}, false
	}
	record, err := s.deferredRenderer.Render(ctx, DeferredRenderRequest{
		TaskID:    req.TaskID,
		Product:   req.Product,
		Task:      task,
		BaseAsset: base,
	})
	if err != nil || record == nil {
		return asset.AssetRecord{}, false
	}
	if strings.TrimSpace(record.TaskID) == "" {
		record.TaskID = req.TaskID
	}
	if strings.TrimSpace(record.Generator) == "" {
		record.Generator = "asset_generation_renderer"
	}
	if record.Metadata == nil {
		record.Metadata = map[string]string{}
	}
	record.Metadata["execution_mode"] = ExecutionModeRendererBacked
	if record.Lineage == nil {
		record.Lineage = &asset.AssetLineage{
			ParentAssetIDs: []string{base.ID},
			SourceAssetIDs: []string{base.ID},
			Step:           "renderer_backed",
		}
	}
	return *record, true
}
