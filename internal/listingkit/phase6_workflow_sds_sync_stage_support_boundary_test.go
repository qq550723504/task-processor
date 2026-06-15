package listingkit

import "testing"

func TestWorkflowSDSSyncStageSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "workflow_sds_sync.go")
	assertSourceContainsAll(t, rootSource, []string{
		"func (s *service) syncSDSDesign(",
		"func (s *service) syncSDSDesignFromRemote(",
		"func (s *service) syncSDSDesignVariantsFromRemote(",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"func normalizeSDSSyncRecorder(",
		"func beginSDSSyncStage(",
		"func failSDSSyncStage(",
		"func finalizeSDSSyncSummary(",
		"func failedSDSVariantSyncSummary(",
		"func emptySDSVariantSyncSummary(",
	})

	supportSource := readTaskGenerationSourceFile(t, "workflow_sds_sync_stage_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"func normalizeSDSSyncRecorder(",
		"func beginSDSSyncStage(",
		"func failSDSSyncStage(",
		"func finalizeSDSSyncSummary(",
		"func failedSDSVariantSyncSummary(",
		"func emptySDSVariantSyncSummary(",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"func (s *service) syncSDSDesign(",
		"func (s *service) syncSDSDesignFromRemote(",
		"func (s *service) syncSDSDesignVariantsFromRemote(",
	})
}
