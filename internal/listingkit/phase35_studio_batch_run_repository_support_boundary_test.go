package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestStudioBatchRunRepositorySupportFilesOwnConcreteImplementations(t *testing.T) {
	t.Parallel()

	homeSrc, err := os.ReadFile("studio_batch_run_repository.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_run_repository.go) error = %v", err)
	}
	homeContent := string(homeSrc)

	for _, needle := range []string{
		"type StudioBatchRunRepository interface {",
		"func applyStudioBatchRunScopeDefaults(ctx context.Context, tenantID *string, userID *string) {",
		"func matchesStudioBatchRunScope(ctx context.Context, tenantID string, userID string) bool {",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("studio_batch_run_repository.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"type MemStudioBatchRunRepository struct {",
		"type GormStudioBatchRunRepository struct {",
		"func NewMemStudioBatchRunRepository() *MemStudioBatchRunRepository {",
		"func NewGormStudioBatchRunRepository(db *gorm.DB) *GormStudioBatchRunRepository {",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("studio_batch_run_repository.go should delegate concrete implementation %q", needle)
		}
	}

	memSrc, err := os.ReadFile("studio_batch_run_repository_mem.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_run_repository_mem.go) error = %v", err)
	}
	memContent := string(memSrc)

	for _, needle := range []string{
		"type MemStudioBatchRunRepository struct {",
		"func NewMemStudioBatchRunRepository() *MemStudioBatchRunRepository {",
		"func (r *MemStudioBatchRunRepository) CreateStudioBatchRun(",
		"func (r *MemStudioBatchRunRepository) UpdateStudioBatchRunItem(",
	} {
		if !strings.Contains(memContent, needle) {
			t.Fatalf("studio_batch_run_repository_mem.go should contain %q", needle)
		}
	}

	gormSrc, err := os.ReadFile("studio_batch_run_repository_gorm.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_run_repository_gorm.go) error = %v", err)
	}
	gormContent := string(gormSrc)

	for _, needle := range []string{
		"type GormStudioBatchRunRepository struct {",
		"func NewGormStudioBatchRunRepository(db *gorm.DB) *GormStudioBatchRunRepository {",
		"func AutoMigrateStudioBatchRunRepository(db *gorm.DB) error {",
		"func (r *GormStudioBatchRunRepository) CreateStudioBatchRun(",
		"func (r *GormStudioBatchRunRepository) UpdateStudioBatchRunItem(",
	} {
		if !strings.Contains(gormContent, needle) {
			t.Fatalf("studio_batch_run_repository_gorm.go should contain %q", needle)
		}
	}
}
