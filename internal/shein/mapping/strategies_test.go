package mapping_test

import (
	"testing"
	"time"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/shein/mapping"
)

func makeRepairCtx(skuCode, spuCode, spuName string, storeInfo *managementapi.StoreRespDTO) *mapping.MappingRepairContext {
	return &mapping.MappingRepairContext{
		Request: &mapping.MappingRepairRequest{
			SkuCode: skuCode,
			SpuCode: spuCode,
			SpuName: spuName,
		},
		StoreInfo: storeInfo,
		StartTime: time.Now(),
	}
}

func makeStoreInfo(region string) *managementapi.StoreRespDTO {
	return &managementapi.StoreRespDTO{Region: region}
}

func TestProductBasedRepairStrategy_CanRepair(t *testing.T) {
	strategy := mapping.NewProductBasedRepairStrategy(nil, nil)

	tests := []struct {
		name string
		ctx  *mapping.MappingRepairContext
		want bool
	}{
		{
			"has_sku_and_store",
			makeRepairCtx("SKU001", "", "", makeStoreInfo("US")),
			true,
		},
		{
			"empty_sku",
			makeRepairCtx("", "", "", makeStoreInfo("US")),
			false,
		},
		{
			"nil_store",
			makeRepairCtx("SKU001", "", "", nil),
			false,
		},
		{
			"empty_sku_and_nil_store",
			makeRepairCtx("", "", "", nil),
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := strategy.CanRepair(tc.ctx)
			if got != tc.want {
				t.Errorf("CanRepair() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProductBasedRepairStrategy_GetStrategyName(t *testing.T) {
	strategy := mapping.NewProductBasedRepairStrategy(nil, nil)
	if got := strategy.GetStrategyName(); got != "ProductBasedRepair" {
		t.Errorf("GetStrategyName() = %q, want %q", got, "ProductBasedRepair")
	}
}

func TestHistoryBasedRepairStrategy_CanRepair(t *testing.T) {
	strategy := mapping.NewHistoryBasedRepairStrategy(nil)

	tests := []struct {
		name string
		ctx  *mapping.MappingRepairContext
		want bool
	}{
		{
			"has_spu_code",
			makeRepairCtx("", "SPU001", "", nil),
			true,
		},
		{
			"has_spu_name",
			makeRepairCtx("", "", "My Product", nil),
			true,
		},
		{
			"has_both",
			makeRepairCtx("SKU001", "SPU001", "My Product", makeStoreInfo("US")),
			true,
		},
		{
			"empty_spu_code_and_name",
			makeRepairCtx("SKU001", "", "", makeStoreInfo("US")),
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := strategy.CanRepair(tc.ctx)
			if got != tc.want {
				t.Errorf("CanRepair() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHistoryBasedRepairStrategy_GetStrategyName(t *testing.T) {
	strategy := mapping.NewHistoryBasedRepairStrategy(nil)
	if got := strategy.GetStrategyName(); got != "HistoryBasedRepair" {
		t.Errorf("GetStrategyName() = %q, want %q", got, "HistoryBasedRepair")
	}
}

func TestSmartRepairStrategy_CanRepair(t *testing.T) {
	strategy := mapping.NewSmartRepairStrategy(nil, nil, nil, nil)

	tests := []struct {
		name string
		ctx  *mapping.MappingRepairContext
		want bool
	}{
		{
			"has_sku_and_store",
			makeRepairCtx("SKU001", "", "", makeStoreInfo("US")),
			true,
		},
		{
			"empty_sku",
			makeRepairCtx("", "", "", makeStoreInfo("US")),
			false,
		},
		{
			"nil_store",
			makeRepairCtx("SKU001", "", "", nil),
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := strategy.CanRepair(tc.ctx)
			if got != tc.want {
				t.Errorf("CanRepair() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSmartRepairStrategy_GetStrategyName(t *testing.T) {
	strategy := mapping.NewSmartRepairStrategy(nil, nil, nil, nil)
	if got := strategy.GetStrategyName(); got != "SmartRepair" {
		t.Errorf("GetStrategyName() = %q, want %q", got, "SmartRepair")
	}
}

func TestDefaultMappingRepairConfig(t *testing.T) {
	cfg := mapping.DefaultMappingRepairConfig()

	if cfg.MaxRetryCount != 3 {
		t.Errorf("MaxRetryCount = %d, want 3", cfg.MaxRetryCount)
	}
	if cfg.BatchSize != 50 {
		t.Errorf("BatchSize = %d, want 50", cfg.BatchSize)
	}
	if !cfg.EnableAutoRepair {
		t.Error("EnableAutoRepair should be true")
	}
	if cfg.RetryInterval != 5*time.Minute {
		t.Errorf("RetryInterval = %v, want 5m", cfg.RetryInterval)
	}
	if cfg.RepairTimeout != 30*time.Second {
		t.Errorf("RepairTimeout = %v, want 30s", cfg.RepairTimeout)
	}
}
