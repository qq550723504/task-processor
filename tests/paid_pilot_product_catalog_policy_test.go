package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPaidPilotProductCatalogFreezesApprovedPolicy(t *testing.T) {
	path := filepath.Join("..", "docs", "product", "listingkit-paid-pilot-product-catalog.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read paid-pilot product catalog: %v", err)
	}

	for _, required := range []string{
		"`paid_pilot`",
		"仅限邀请制",
		"`shein_publish`",
		"默认关闭",
		"1688",
		"保持关闭",
		"studio_design_jobs_succeeded",
		"product_image_jobs_succeeded",
		"shein_drafts_succeeded",
		"shein_publishes_succeeded",
		"storage_bytes_current",
		"失败、取消、平台拒绝和工程重放不计费",
	} {
		if !strings.Contains(string(content), required) {
			t.Errorf("paid-pilot product catalog must contain %q", required)
		}
	}
}
