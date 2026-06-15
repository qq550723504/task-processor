package listingkit

import "testing"

func TestStudioBatchRepositoryMemSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "studio_batch_repository_mem.go")
	assertSourceContainsAll(t, rootSource, []string{
		"type MemStudioBatchRepository struct {",
		"func NewMemStudioBatchRepository() *MemStudioBatchRepository {",
		"func (r *MemStudioBatchRepository) CreateStudioBatchGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord, designs []StudioMaterializedDesignRecord) error {",
		"func (r *MemStudioBatchRepository) CreateStudioBatchItems(ctx context.Context, batchID string, items []StudioBatchItemRecord) error {",
		"func (r *MemStudioBatchRepository) CreateStudioGenerationAttempt(ctx context.Context, attempt *StudioGenerationAttemptRecord) error {",
		"func (r *MemStudioBatchRepository) ClaimStudioBatchItem(ctx context.Context, itemID string, fromStatus StudioBatchItemStatus, toStatus StudioBatchItemStatus, updatedAt time.Time) (*StudioBatchItemRecord, bool, error) {",
		"func (r *MemStudioBatchRepository) GetStudioBatch(ctx context.Context, batchID string) (*StudioBatchRecord, error) {",
		"func (r *MemStudioBatchRepository) GetStudioBatchItem(ctx context.Context, itemID string) (*StudioBatchItemRecord, error) {",
		"func (r *MemStudioBatchRepository) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetailGraph, error) {",
		"func (r *MemStudioBatchRepository) ListStudioMaterializedDesignsByIDs(ctx context.Context, batchID string, designIDs []string) ([]StudioMaterializedDesignRecord, error) {",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"func (r *MemStudioBatchRepository) ReplaceStudioBatchGenerationGraph(",
		"func (r *MemStudioBatchRepository) ReplaceStudioMaterializedDesignReviews(",
		"func (r *MemStudioBatchRepository) UpdateStudioBatch(",
		"func (r *MemStudioBatchRepository) UpdateStudioMaterializedDesign(",
		"func (r *MemStudioBatchRepository) buildDetailGraphLocked(",
	})

	supportSource := readTaskGenerationSourceFile(t, "studio_batch_repository_mem_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"func (r *MemStudioBatchRepository) ReplaceStudioBatchGenerationGraph(",
		"func (r *MemStudioBatchRepository) ReplaceStudioMaterializedDesignReviews(",
		"func (r *MemStudioBatchRepository) UpdateStudioBatch(",
		"func (r *MemStudioBatchRepository) UpdateStudioMaterializedDesign(",
		"func (r *MemStudioBatchRepository) buildDetailGraphLocked(",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"func NewMemStudioBatchRepository() *MemStudioBatchRepository {",
		"func (r *MemStudioBatchRepository) CreateStudioBatchGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord, designs []StudioMaterializedDesignRecord) error {",
		"func (r *MemStudioBatchRepository) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetailGraph, error) {",
		"func (r *MemStudioBatchRepository) ListStudioMaterializedDesignsByIDs(ctx context.Context, batchID string, designIDs []string) ([]StudioMaterializedDesignRecord, error) {",
	})
}
