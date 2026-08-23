package listingkit

import (
	"context"
	"errors"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	"task-processor/internal/catalog"
	listingplatform "task-processor/internal/listing/platform"
)

func dispatchGenerationTasksByPlatform(
	ctx context.Context,
	generator assetgeneration.Service,
	taskID string,
	product *catalog.Product,
	listingResult *ListingKitResult,
	sharedInventory *asset.Inventory,
	tasks []assetgeneration.Task,
) (*assetgeneration.Result, error) {
	if generator == nil || len(tasks) == 0 {
		return nil, nil
	}

	groups := make(map[string][]assetgeneration.Task, len(tasks))
	groupOrder := make([]string, 0, len(tasks))
	for _, task := range tasks {
		group := listingplatform.Normalize(task.Platform)
		if group == "" {
			group = "__shared__"
		}
		if _, ok := groups[group]; !ok {
			groupOrder = append(groupOrder, group)
		}
		groups[group] = append(groups[group], assetgeneration.CloneTask(task))
	}

	updatedByID := make(map[string]assetgeneration.Task, len(tasks))
	assets := make([]asset.AssetRecord, 0)
	var dispatchErr error
	gotResult := false
	for _, group := range groupOrder {
		dispatchInventory := sharedInventory
		if group != "__shared__" && group != "common" {
			dispatchInventory = platformAssetInventory(listingResult, group, sharedInventory)
		}
		dispatchResult, err := generator.Dispatch(ctx, assetgeneration.DispatchRequest{
			TaskID:    taskID,
			Product:   product,
			Inventory: dispatchInventory,
			Tasks:     groups[group],
		})
		dispatchErr = errors.Join(dispatchErr, err)
		if dispatchResult == nil {
			continue
		}
		gotResult = true
		for _, task := range dispatchResult.Tasks {
			updatedByID[task.ID] = assetgeneration.CloneTask(task)
		}
		assets = append(assets, dispatchResult.Assets...)
	}

	if !gotResult {
		return nil, dispatchErr
	}
	mergedTasks := assetgeneration.CloneTasks(tasks)
	for idx, task := range mergedTasks {
		if updated, ok := updatedByID[task.ID]; ok {
			mergedTasks[idx] = updated
		}
	}
	result := &assetgeneration.Result{Tasks: mergedTasks, Assets: assets}
	if dispatchErr != nil {
		result.Error = dispatchErr.Error()
	}
	return result, dispatchErr
}
