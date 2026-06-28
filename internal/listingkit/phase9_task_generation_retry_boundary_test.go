package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestTaskGenerationServiceFileKeepsRetryOwnershipBoundaries(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("task_generation_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_generation_service.go) error = %v", err)
	}
	source := string(content)
	start := strings.Index(source, "func (s *taskGenerationService) RetryTaskGenerationTasks(")
	end := strings.Index(source, "func (s *taskGenerationService) ExecuteTaskGenerationAction(")
	if start == -1 || end == -1 || end <= start {
		t.Fatalf("task_generation_service.go should contain retry generation service boundaries")
	}
	retrySource := source[start:end]

	required := []string{
		"return buildRetryGenerationProjectionPhase(s.assetRecipeResolver, s.assetBundleBuilder).emptySelectionPage(task), nil",
		"updatedTasks := buildRetryGenerationMutationPhase().run(",
		"if err := buildRetryGenerationPersistPhase(s.assetRepo).run(ctx, task.ID, inventory, updatedTasks); err != nil {",
		"rebuiltResult, page := buildRetryGenerationProjectionPhase(s.assetRecipeResolver, s.assetBundleBuilder).run(",
		"if err := s.repo.SaveTaskResult(ctx, task.ID, rebuiltResult); err != nil {",
	}
	for _, needle := range required {
		if !strings.Contains(source, needle) {
			t.Fatalf("task_generation_service.go should contain %q", needle)
		}
	}

	forbidden := []string{
		"mergeGenerationTasks(existingTasks, dispatchResult.Tasks)",
		"replaceGeneratedAssetsForTargets(",
		"asset.RebuildInventorySummary(inventory)",
		"if err := s.assetRepo.SaveInventory(ctx, inventory); err != nil {",
		"if err := s.assetRepo.SaveGenerationTasks(ctx, task.ID, updatedTasks); err != nil {",
		"asset.RebuildBundleFromInventory(",
		"decorateListingKitResultGeneration(&rebuiltResult, updatedTasks)",
		"syncAssetRenderPreviews(&rebuiltResult)",
		"decorateListingKitResultReview(&rebuiltResult, reviews)",
		"attachPlatformImageBundles(&rebuiltResult, inventory, recipesByPlatform, &assetgeneration.Result{Tasks: updatedTasks}, s.assetBundleBuilder)",
		"buildMatchedGenerationQueue(",
		"buildGenerationTaskPage(",
	}
	for _, needle := range forbidden {
		if strings.Contains(retrySource, needle) {
			t.Fatalf("task_generation_service.go should not inline retry ownership %q", needle)
		}
	}
}

func TestRetryGenerationSeamFilesOwnTheirResponsibilities(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		file        string
		shouldOwn   []string
		shouldAvoid []string
	}{
		{
			file: "task_generation_retry_mutation.go",
			shouldOwn: []string{
				"mergeGenerationTasks(",
				"listinggeneration.ReplaceGeneratedAssetsForTargets(",
				"asset.RebuildInventorySummary(",
			},
			shouldAvoid: []string{
				"SaveInventory(",
				"SaveGenerationTasks(",
				"decorateListingKitResultGeneration(",
				"attachPlatformImageBundles(",
				"buildGenerationTaskPage(",
			},
		},
		{
			file: "task_generation_retry_persist.go",
			shouldOwn: []string{
				"SaveInventory(",
				"SaveGenerationTasks(",
			},
			shouldAvoid: []string{
				"mergeGenerationTasks(",
				"replaceGeneratedAssetsForTargets(",
				"asset.RebuildInventorySummary(",
				"decorateListingKitResultGeneration(",
				"attachPlatformImageBundles(",
				"buildGenerationTaskPage(",
			},
		},
		{
			file: "task_generation_retry_projection.go",
			shouldOwn: []string{
				"asset.RebuildBundleFromInventory(",
				"attachPlatformImageBundles(",
				"decorateListingKitResultGeneration(",
				"syncAssetRenderPreviews(",
				"decorateListingKitResultReview(",
				"buildGenerationTaskPage(",
				"buildMatchedGenerationQueue(",
			},
			shouldAvoid: []string{
				"SaveInventory(",
				"SaveGenerationTasks(",
				"mergeGenerationTasks(",
				"replaceGeneratedAssetsForTargets(",
				"asset.RebuildInventorySummary(",
			},
		},
	} {
		content, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", tc.file, err)
		}
		source := string(content)

		for _, needle := range tc.shouldOwn {
			if !strings.Contains(source, needle) {
				t.Fatalf("%s should contain %q", tc.file, needle)
			}
		}
		for _, needle := range tc.shouldAvoid {
			if strings.Contains(source, needle) {
				t.Fatalf("%s should not contain %q", tc.file, needle)
			}
		}
	}
}
