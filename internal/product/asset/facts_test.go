package asset_test

import (
	"testing"

	"task-processor/internal/product/asset"
)

func TestFactsReportsWhetherSourceAssetsExist(t *testing.T) {
	t.Parallel()

	if (asset.Facts{}).HasAssets() {
		t.Fatal("empty source facts reported assets")
	}
	if !(asset.Facts{Items: []asset.ItemFacts{{SourceID: "source-1"}}}).HasAssets() {
		t.Fatal("source facts with an item reported no assets")
	}
}
