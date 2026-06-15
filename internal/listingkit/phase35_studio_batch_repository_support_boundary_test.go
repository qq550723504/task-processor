package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestStudioBatchRepositorySupportFilesOwnConcreteImplementations(t *testing.T) {
	t.Parallel()

	homeSrc, err := os.ReadFile("studio_batch_repository.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_repository.go) error = %v", err)
	}
	homeContent := string(homeSrc)

	for _, needle := range []string{
		"var ErrStudioBatchUnknownItemReference = studiodomain.ErrBatchUnknownItemReference",
		"var ErrStudioBatchOwnershipConflict = studiodomain.ErrBatchOwnershipConflict",
		"type StudioBatchRepository = studiodomain.BatchRepository[",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("studio_batch_repository.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"type MemStudioBatchRepository struct {",
		"type GormStudioBatchRepository struct {",
		"func NewMemStudioBatchRepository() *MemStudioBatchRepository {",
		"func NewGormStudioBatchRepository(db *gorm.DB) *GormStudioBatchRepository {",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("studio_batch_repository.go should delegate concrete implementation %q", needle)
		}
	}

	memSrc, err := os.ReadFile("studio_batch_repository_mem.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_repository_mem.go) error = %v", err)
	}
	memContent := string(memSrc)

	for _, needle := range []string{
		"type MemStudioBatchRepository struct {",
		"func NewMemStudioBatchRepository() *MemStudioBatchRepository {",
		"func (r *MemStudioBatchRepository) CreateStudioBatchGraph(",
		"func (r *MemStudioBatchRepository) GetStudioBatchDetail(",
	} {
		if !strings.Contains(memContent, needle) {
			t.Fatalf("studio_batch_repository_mem.go should contain %q", needle)
		}
	}

	memSupportSrc, err := os.ReadFile("studio_batch_repository_mem_support.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_repository_mem_support.go) error = %v", err)
	}
	memSupportContent := string(memSupportSrc)

	for _, needle := range []string{
		"func (r *MemStudioBatchRepository) ReplaceStudioBatchGenerationGraph(",
		"func (r *MemStudioBatchRepository) ReplaceStudioMaterializedDesignReviews(",
		"func (r *MemStudioBatchRepository) UpdateStudioBatch(",
		"func (r *MemStudioBatchRepository) UpdateStudioBatchItem(",
		"func (r *MemStudioBatchRepository) UpdateStudioGenerationAttempt(",
		"func (r *MemStudioBatchRepository) UpdateStudioMaterializedDesign(",
		"func (r *MemStudioBatchRepository) buildDetailGraphLocked(",
	} {
		if !strings.Contains(memSupportContent, needle) {
			t.Fatalf("studio_batch_repository_mem_support.go should contain %q", needle)
		}
	}

	gormSrc, err := os.ReadFile("studio_batch_repository_gorm.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_repository_gorm.go) error = %v", err)
	}
	gormContent := string(gormSrc)

	for _, needle := range []string{
		"type GormStudioBatchRepository struct {",
		"func NewGormStudioBatchRepository(db *gorm.DB) *GormStudioBatchRepository {",
		"func AutoMigrateStudioBatchRepository(db *gorm.DB) error {",
		"func (r *GormStudioBatchRepository) CreateStudioBatchGraph(",
		"func (r *GormStudioBatchRepository) GetStudioBatchDetail(",
	} {
		if !strings.Contains(gormContent, needle) {
			t.Fatalf("studio_batch_repository_gorm.go should contain %q", needle)
		}
	}
}
