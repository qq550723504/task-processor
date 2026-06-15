package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestStudioBatchRepositoryGormUpdateSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSrc, err := os.ReadFile("studio_batch_repository_gorm.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_repository_gorm.go) error = %v", err)
	}
	supportSrc, err := os.ReadFile("studio_batch_repository_gorm_update_support.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_repository_gorm_update_support.go) error = %v", err)
	}

	rootContent := string(rootSrc)
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func (r *GormStudioBatchRepository) CreateStudioBatchGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord, designs []StudioMaterializedDesignRecord) error {",
		"func (r *GormStudioBatchRepository) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetailGraph, error) {",
		"func (r *GormStudioBatchRepository) ListStudioMaterializedDesignsByIDs(ctx context.Context, batchID string, designIDs []string) ([]StudioMaterializedDesignRecord, error) {",
	} {
		if !strings.Contains(rootContent, needle) {
			t.Fatalf("studio_batch_repository_gorm.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (r *GormStudioBatchRepository) ReplaceStudioItemMaterializedDesigns(ctx context.Context, itemID string, designs []StudioMaterializedDesignRecord) error {",
		"func (r *GormStudioBatchRepository) ReplaceStudioMaterializedDesignReviews(ctx context.Context, batchID string, designIDs []string, updatedAt time.Time) error {",
		"func (r *GormStudioBatchRepository) UpdateStudioBatch(ctx context.Context, batch *StudioBatchRecord) error {",
		"func (r *GormStudioBatchRepository) UpdateStudioBatchItem(ctx context.Context, item *StudioBatchItemRecord) error {",
		"func (r *GormStudioBatchRepository) UpdateStudioGenerationAttempt(ctx context.Context, attempt *StudioGenerationAttemptRecord) error {",
		"func (r *GormStudioBatchRepository) UpdateStudioMaterializedDesign(ctx context.Context, design *StudioMaterializedDesignRecord) error {",
	} {
		if strings.Contains(rootContent, needle) {
			t.Fatalf("studio_batch_repository_gorm.go should delegate update helper %q", needle)
		}
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("studio_batch_repository_gorm_update_support.go should contain %q", needle)
		}
	}
}
