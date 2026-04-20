package generation

import (
	"context"
	"fmt"

	"task-processor/internal/asset"
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/productimage"
)

type Config struct {
	SubjectExtractor        productimage.SubjectExtractor
	WhiteBackgroundRenderer productimage.WhiteBackgroundRenderer
	DeferredRenderer        DeferredRenderer
}

type service struct {
	subjectExtractor        productimage.SubjectExtractor
	whiteBackgroundRenderer productimage.WhiteBackgroundRenderer
	deferredRenderer        DeferredRenderer
}

func NewService(config Config) Service {
	return &service{
		subjectExtractor:        config.SubjectExtractor,
		whiteBackgroundRenderer: config.WhiteBackgroundRenderer,
		deferredRenderer:        config.DeferredRenderer,
	}
}

func NewNoopService() Service {
	return NewService(Config{})
}

func (s *service) Plan(ctx context.Context, req Request) (*Result, error) {
	result := &Result{}
	for idx, item := range req.Recipes {
		if !item.Generated {
			continue
		}
		if recipeAlreadySatisfied(req.Inventory, item) {
			continue
		}
		result.Tasks = append(result.Tasks, Task{
			TaskID:          req.TaskID,
			ID:              fmt.Sprintf("%s-%d", item.Platform, idx+1),
			Platform:        item.Platform,
			RecipeID:        item.ID,
			AssetKind:       item.AssetKind,
			Slot:            recipeSlot(item),
			Purpose:         recipePurpose(item),
			Status:          "planned",
			ExecutionStatus: "planned",
			ExecutionMode:   PlannedExecutionMode(item.AssetKind),
			CanExecute:      item.Generated,
			Lineage:         plannedLineage(item),
			SourceAssetIDs:  candidateSourceAssetIDs(req.Inventory),
		})
	}
	return result, nil
}

func (s *service) Execute(ctx context.Context, req Request) (*Result, error) {
	result := &Result{}
	for idx, item := range req.Recipes {
		if !item.Generated {
			continue
		}
		if recipeAlreadySatisfied(req.Inventory, item) {
			continue
		}
		record, ok := s.executeRecipe(ctx, req, idx, item)
		if !ok {
			continue
		}
		result.Tasks = append(result.Tasks, Task{
			TaskID:           req.TaskID,
			ID:               fmt.Sprintf("%s-exec-%d", item.Platform, idx+1),
			Platform:         item.Platform,
			RecipeID:         item.ID,
			AssetKind:        item.AssetKind,
			Slot:             recipeSlot(item),
			Purpose:          recipePurpose(item),
			Status:           "completed",
			ExecutionStatus:  "completed",
			ExecutionMode:    record.Metadata["execution_mode"],
			CanExecute:       true,
			SatisfiedBy:      "generated_asset",
			Lineage:          plannedLineage(item),
			SourceAssetIDs:   sourceAssetIDsForRecord(record),
			Metadata:         taskMetadataFromAssetMetadata(record.Metadata),
			ReviewConfidence: reviewConfidenceFromMetadata(record.Metadata),
		})
		result.Assets = append(result.Assets, record)
	}
	return result, nil
}

func (s *service) Dispatch(ctx context.Context, req DispatchRequest) (*Result, error) {
	result := &Result{}
	for idx, task := range req.Tasks {
		updated, produced := s.dispatchTask(ctx, req, idx, task)
		result.Tasks = append(result.Tasks, updated)
		result.Assets = append(result.Assets, produced...)
	}
	return result, nil
}

func (s *service) executeRecipe(ctx context.Context, req Request, idx int, item assetrecipe.AssetRecipe) (asset.AssetRecord, bool) {
	switch item.AssetKind {
	case asset.KindWhiteBgImage:
		if record, ok := s.executeWhiteBackground(ctx, req, idx, item); ok {
			return record, true
		}
	case asset.KindSubjectCutout:
		if record, ok := s.executeSubjectCutout(ctx, req, idx, item); ok {
			return record, true
		}
	}
	return executeNativeRecipe(req.TaskID, idx, req.Inventory, item)
}

func (s *service) dispatchTask(ctx context.Context, req DispatchRequest, idx int, task Task) (Task, []asset.AssetRecord) {
	updated := task
	if !task.CanExecute || task.ExecutionStatus == "completed" {
		return updated, nil
	}
	if task.ExecutionMode == ExecutionModeRendererBacked && s.deferredRenderer != nil {
		record, ok := s.executeRendererBackedTask(ctx, req, task)
		if ok {
			updated.Status = "completed"
			updated.ExecutionStatus = "completed"
			updated.ExecutionMode = ExecutionModeRendererBacked
			updated.SatisfiedBy = ExecutionModeGeneratedAsset
			updated.Metadata = taskMetadataFromAssetMetadata(record.Metadata)
			updated.ReviewConfidence = reviewConfidenceFromMetadata(record.Metadata)
			return updated, []asset.AssetRecord{record}
		}
	}
	if task.ExecutionMode != ExecutionModeDeferredPlan && task.ExecutionMode != ExecutionModeRendererBacked {
		return updated, nil
	}
	record, ok := executeDeferredTask(req.TaskID, idx, req.Inventory, task)
	if !ok {
		return updated, nil
	}
	updated.Status = "completed"
	updated.ExecutionStatus = "completed"
	updated.ExecutionMode = ExecutionModeDeferredStub
	updated.SatisfiedBy = ExecutionModeGeneratedAsset
	updated.Metadata = taskMetadataFromAssetMetadata(record.Metadata)
	updated.ReviewConfidence = reviewConfidenceFromMetadata(record.Metadata)
	return updated, []asset.AssetRecord{record}
}
