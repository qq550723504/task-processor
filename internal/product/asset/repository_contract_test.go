package asset_test

import (
	"testing"

	"task-processor/internal/product/asset"
	"task-processor/internal/product/asset/assettest"
)

func TestMemoryRepositoryContract(t *testing.T) {
	assettest.ExerciseRepositoryContract(t, func(t *testing.T) asset.Repository {
		t.Helper()
		return assettest.NewMemoryRepository()
	})
}
