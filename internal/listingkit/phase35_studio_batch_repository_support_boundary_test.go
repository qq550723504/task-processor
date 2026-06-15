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
		"type StudioBatchRepository interface {",
		"var ErrStudioBatchUnknownItemReference = errors.New(\"studio batch graph references unknown item\")",
		"var ErrStudioBatchOwnershipConflict = errors.New(\"studio batch update conflicts with immutable ownership\")",
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
		"func (r *MemStudioBatchRepository) buildDetailGraphLocked(",
	} {
		if !strings.Contains(memContent, needle) {
			t.Fatalf("studio_batch_repository_mem.go should contain %q", needle)
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
		"func (r *GormStudioBatchRepository) UpdateStudioMaterializedDesign(",
	} {
		if !strings.Contains(gormContent, needle) {
			t.Fatalf("studio_batch_repository_gorm.go should contain %q", needle)
		}
	}
}
