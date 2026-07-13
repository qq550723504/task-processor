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
		"唯一首发套餐代码为 `paid_pilot`",
		"Basic 和 Professional 不作为首发套餐",
		"`paid_pilot`",
		"仅限邀请制",
		"独立 `shein_publish` entitlement",
		"`shein_publish`",
		"默认关闭",
		"1688",
		"保持关闭",
		"studio_design_jobs_succeeded",
		"product_image_jobs_succeeded",
		"shein_drafts_succeeded",
		"shein_publishes_succeeded",
		"storage_bytes_current",
		"当前保留对象的字节占用",
		"失败、取消、平台拒绝和工程重放不计费",
		"只有 PAY-041 的幂等 usage ledger、PAY-042 的统一入口执行、PAY-043 的人工商业台账和 PAY-044 的对账/补记能力完成",
	} {
		if !strings.Contains(string(content), required) {
			t.Errorf("paid-pilot product catalog must contain %q", required)
		}
	}
}
