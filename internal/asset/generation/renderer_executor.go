package generation

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/asset"
)

func (s *service) executeRendererBackedTask(ctx context.Context, req DispatchRequest, task Task) (asset.AssetRecord, bool, error) {
	if s.deferredRenderer == nil {
		return asset.AssetRecord{}, false, nil
	}
	base, ok := preferredDeferredBaseRecord(req.Inventory, task)
	if !ok {
		return asset.AssetRecord{}, false, fmt.Errorf("renderer-backed task %q has no eligible base asset", task.ID)
	}
	record, err := s.deferredRenderer.Render(ctx, DeferredRenderRequest{
		TaskID:    req.TaskID,
		Product:   req.Product,
		Task:      task,
		BaseAsset: base,
	})
	if err != nil {
		return asset.AssetRecord{}, false, fmt.Errorf("renderer-backed task %q failed: %w", task.ID, err)
	}
	if record == nil {
		return asset.AssetRecord{}, false, fmt.Errorf("renderer-backed task %q returned no asset", task.ID)
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
	return *record, true, nil
}
