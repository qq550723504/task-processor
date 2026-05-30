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

	required := []string{
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
		"rebuildInventorySummary(inventory)",
		"if err := s.assetRepo.SaveInventory(ctx, inventory); err != nil {",
		"if err := s.assetRepo.SaveGenerationTasks(ctx, task.ID, updatedTasks); err != nil {",
		"decorateListingKitResultGeneration(&rebuiltResult, updatedTasks)",
		"attachPlatformImageBundles(&rebuiltResult, inventory, recipesByPlatform, &assetgeneration.Result{Tasks: updatedTasks}, s.assetBundleBuilder)",
	}
	for _, needle := range forbidden {
		if strings.Contains(source, needle) {
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
				"replaceGeneratedAssetsForTargets(",
				"rebuildInventorySummary(",
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
				"rebuildInventorySummary(",
				"decorateListingKitResultGeneration(",
				"attachPlatformImageBundles(",
				"buildGenerationTaskPage(",
			},
		},
		{
			file: "task_generation_retry_projection.go",
			shouldOwn: []string{
				"rebuildBundleFromInventory(",
				"attachPlatformImageBundles(",
				"decorateListingKitResultGeneration(",
				"buildGenerationTaskPage(",
			},
			shouldAvoid: []string{
				"SaveInventory(",
				"SaveGenerationTasks(",
				"mergeGenerationTasks(",
				"replaceGeneratedAssetsForTargets(",
				"rebuildInventorySummary(",
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
