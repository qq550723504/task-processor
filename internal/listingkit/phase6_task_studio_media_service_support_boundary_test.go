package listingkit

import "testing"

func TestTaskStudioMediaServiceSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "task_studio_media_service.go")
	assertSourceContainsAll(t, rootSource, []string{
		"func newTaskStudioMediaService(",
		"func (s *taskStudioMediaService) GenerateStudioDesigns(",
		"func (s *taskStudioMediaService) GenerateStudioProductImages(",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"func (s *taskStudioMediaService) generateStudioDesignSiblingThemes(",
		"func (s *taskStudioMediaService) generateStudioDesignImage(",
		"func (s *taskStudioMediaService) editStudioDesignImageWithReferences(",
		"func (s *taskStudioMediaService) generateStudioDesignImageWithoutReferences(",
		"func (s *taskStudioMediaService) persistGeneratedStudioImage(",
		"func (s *taskStudioMediaService) generateOneStudioProductImage(",
		"func (s *taskStudioMediaService) tryGenerateStudioProductImage(",
		"func (s *taskStudioMediaService) sanitizeStudioImageInputURLs(",
	})

	supportSource := readTaskGenerationSourceFile(t, "task_studio_media_service_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"func (s *taskStudioMediaService) generateStudioDesignSiblingThemes(",
		"func (s *taskStudioMediaService) generateStudioDesignImage(",
		"func (s *taskStudioMediaService) editStudioDesignImageWithReferences(",
		"func (s *taskStudioMediaService) generateStudioDesignImageWithoutReferences(",
		"func (s *taskStudioMediaService) persistGeneratedStudioImage(",
		"func (s *taskStudioMediaService) generateOneStudioProductImage(",
		"func (s *taskStudioMediaService) tryGenerateStudioProductImage(",
		"func (s *taskStudioMediaService) sanitizeStudioImageInputURLs(",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"func newTaskStudioMediaService(",
		"func (s *taskStudioMediaService) GenerateStudioDesigns(",
		"func (s *taskStudioMediaService) GenerateStudioProductImages(",
	})
}
