package sheinsync

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSheinCandidateServiceRefreshCandidatesSkipsProductsWithoutEffectiveCostForEligibleCount(t *testing.T) {
	t.Parallel()

	repo := newSheinCandidateRepoStub([]SheinSyncedProductRecord{
		{
			ID:                 1,
			TenantID:           11,
			StoreID:            22,
			SKCName:            "eligible-skc",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(12.5),
			PriceSnapshot:      `{"sale_price":29.9}`,
			InventorySnapshot:  `{"available":18}`,
			IsActive:           true,
		},
		{
			ID:                2,
			TenantID:          11,
			StoreID:           22,
			SKCName:           "missing-cost-skc",
			ShelfStatus:       "ON_SHELF",
			PriceSnapshot:     `{"sale_price":19.9}`,
			InventorySnapshot: `{"available":9}`,
			IsActive:          true,
		},
	})

	service := NewSheinCandidateService(repo)

	result, err := service.RefreshCandidates(context.Background(), 11, 22, "flash_sale")
	require.NoError(t, err)
	require.Equal(t, 1, result.EligibleCount)
	require.Equal(t, 2, result.TotalCount)
	require.Len(t, repo.savedCandidates(), 2)

	candidates := repo.savedCandidates()
	require.Equal(t, SheinCandidateEligibilityStatusEligible, candidates[0].EligibilityStatus)
	require.Equal(t, SheinCandidateEligibilityStatusIneligible, candidates[1].EligibilityStatus)
	require.NotEmpty(t, candidates[1].EligibilityReason)
}

func TestSheinCandidateServiceRefreshCandidatesUsesListingKitMirrorRowsScopedByTenantStore(t *testing.T) {
	t.Parallel()

	repo := newSheinCandidateRepoStub([]SheinSyncedProductRecord{
		{
			ID:                 10,
			TenantID:           1,
			StoreID:            2,
			SKCName:            "target-skc",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(8.8),
			IsActive:           true,
		},
		{
			ID:                 11,
			TenantID:           1,
			StoreID:            99,
			SKCName:            "other-store",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(7.7),
			IsActive:           true,
		},
		{
			ID:                 12,
			TenantID:           99,
			StoreID:            2,
			SKCName:            "other-tenant",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(6.6),
			IsActive:           true,
		},
	})

	service := NewSheinCandidateService(repo)

	result, err := service.RefreshCandidates(context.Background(), 1, 2, "flash_sale")
	require.NoError(t, err)
	require.Equal(t, 1, result.EligibleCount)
	require.Len(t, repo.queries, 1)
	require.Equal(t, int64(1), repo.queries[0].TenantID)
	require.Equal(t, int64(2), repo.queries[0].StoreID)
	require.NotNil(t, repo.queries[0].IsActive)
	require.True(t, *repo.queries[0].IsActive)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 1)
	require.Equal(t, int64(10), candidates[0].SyncedProductID)
	require.Equal(t, "target-skc", candidates[0].SKCName)
}

func TestSheinCandidateServiceRefreshCandidatesPersistsPendingReviewEligibleCandidatesWithDeterministicFields(t *testing.T) {
	t.Parallel()

	repo := newSheinCandidateRepoStub([]SheinSyncedProductRecord{
		{
			ID:                 5,
			TenantID:           11,
			StoreID:            22,
			SKCName:            "stable-skc",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(15.5),
			PriceSnapshot:      `{"sale_price":39.9}`,
			InventorySnapshot:  `{"available":28}`,
			SyncVersion:        "sync-v1",
			IsActive:           true,
		},
	})

	service := NewSheinCandidateService(repo)

	first, err := service.RefreshCandidates(context.Background(), 11, 22, "flash_sale")
	require.NoError(t, err)
	require.Equal(t, 1, first.EligibleCount)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 1)
	firstCandidate := candidates[0]
	require.Equal(t, SheinCandidateEligibilityStatusEligible, firstCandidate.EligibilityStatus)
	require.Equal(t, SheinCandidateReviewStatusPendingReview, firstCandidate.ReviewStatus)
	require.False(t, firstCandidate.AutoModeEligible)
	require.Equal(t, "flash_sale:11:22", firstCandidate.ActivityKey)
	require.Equal(t, 15.5, *firstCandidate.EffectiveCostPrice)
	require.Equal(t, `{"sale_price":39.9}`, firstCandidate.PriceSnapshot)
	require.Equal(t, `{"available":28}`, firstCandidate.InventorySnapshot)
	require.NotEmpty(t, firstCandidate.CandidateVersion)

	second, err := service.RefreshCandidates(context.Background(), 11, 22, "flash_sale")
	require.NoError(t, err)
	require.Equal(t, 1, second.EligibleCount)

	candidates = repo.savedCandidates()
	require.Len(t, candidates, 1)
	require.Equal(t, firstCandidate.ActivityKey, candidates[0].ActivityKey)
	require.Equal(t, firstCandidate.CandidateVersion, candidates[0].CandidateVersion)
}

func TestSheinCandidateServiceRefreshCandidatesUsesSharedSDSCostForSameSupplierCode(t *testing.T) {
	t.Parallel()

	repo := newSheinCandidateRepoStub([]SheinSyncedProductRecord{
		{
			ID:                 101,
			TenantID:           11,
			StoreID:            22,
			SKCName:            "canvas-a",
			SupplierCode:       "MG8006905001-B3195DA6",
			ShelfStatus:        "ON_SHELF",
			ManualCostPrice:    float64Ptr(46.8),
			AutoCostPrice:      float64Ptr(39.9),
			EffectiveCostPrice: float64Ptr(46.8),
			CostPriceSource:    SheinCostPriceSourceManual,
			PriceSnapshot:      `{"sale_price":78}`,
			InventorySnapshot:  `{"available":9}`,
			IsActive:           true,
		},
		{
			ID:                 102,
			TenantID:           11,
			StoreID:            22,
			SKCName:            "canvas-b",
			SupplierCode:       " MG8006905001-B3195DA6 ",
			ShelfStatus:        "ON_SHELF",
			AutoCostPrice:      float64Ptr(41.2),
			EffectiveCostPrice: float64Ptr(41.2),
			CostPriceSource:    SheinCostPriceSourceAuto,
			PriceSnapshot:      `{"sale_price":80}`,
			InventorySnapshot:  `{"available":11}`,
			IsActive:           true,
		},
	})

	service := NewSheinCandidateService(repo)

	result, err := service.RefreshCandidates(context.Background(), 11, 22, "PROMOTION")
	require.NoError(t, err)
	require.Equal(t, 2, result.EligibleCount)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 2)
	require.Equal(t, "canvas-a", candidates[0].SKCName)
	require.Equal(t, "canvas-b", candidates[1].SKCName)
	require.NotNil(t, candidates[0].EffectiveCostPrice)
	require.NotNil(t, candidates[1].EffectiveCostPrice)
	require.Equal(t, 46.8, *candidates[0].EffectiveCostPrice)
	require.Equal(t, 46.8, *candidates[1].EffectiveCostPrice)
}

func TestSheinCandidateServiceRefreshCandidatesCalculatesProfitRateFromSharedSDSCost(t *testing.T) {
	t.Parallel()

	repo := newSheinCandidateRepoStub([]SheinSyncedProductRecord{
		{
			ID:                 201,
			TenantID:           11,
			StoreID:            22,
			SKCName:            "group-cost-source",
			SupplierCode:       "MG8006905002-B3195DA6",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(30),
			PriceSnapshot:      `{"sale_price":60}`,
			InventorySnapshot:  `{"available":5}`,
			IsActive:           true,
		},
		{
			ID:                  202,
			TenantID:            11,
			StoreID:             22,
			SKCName:             "group-cost-target",
			SupplierCode:        "MG8006905002-B3195DA6",
			ShelfStatus:         "ON_SHELF",
			EffectiveCostPrice:  float64Ptr(20),
			SupplyPrice:         float64Ptr(80),
			SupplyPriceCurrency: "USD",
			PriceSnapshot:       `{"sale_price":100}`,
			InventorySnapshot:   `{"available":6}`,
			IsActive:            true,
		},
	})

	service := NewSheinCandidateService(repo)

	_, err := service.RefreshCandidates(context.Background(), 11, 22, "PROMOTION")
	require.NoError(t, err)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 2)
	require.NotNil(t, candidates[1].CalculatedProfitRate)
	require.Equal(t, 0.625, *candidates[1].CalculatedProfitRate)
	price, currency := parsePromotionPriceSnapshot(candidates[1].PriceSnapshot)
	require.Equal(t, 80.0, price)
	require.Equal(t, "USD", currency)
}

func TestSheinCandidateServiceRefreshCandidatesUsesSharedSDSCostForSameSourceSDS(t *testing.T) {
	t.Parallel()

	repo := newSheinCandidateRepoStub([]SheinSyncedProductRecord{
		{
			ID:                 301,
			TenantID:           11,
			StoreID:            22,
			SKCName:            "canvas-large",
			SupplierCode:       "MG8006905001-B3195DA6",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(46.8),
			PriceSnapshot:      `{"sale_price":90}`,
			InventorySnapshot:  `{"available":8}`,
			IsActive:           true,
		},
		{
			ID:                 302,
			TenantID:           11,
			StoreID:            22,
			SKCName:            "canvas-small",
			SupplierCode:       "MG8006905001-C3195DA6",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(39.1),
			PriceSnapshot:      `{"sale_price":88}`,
			InventorySnapshot:  `{"available":7}`,
			IsActive:           true,
		},
	})

	service := NewSheinCandidateService(repo)

	_, err := service.RefreshCandidates(context.Background(), 11, 22, "PROMOTION")
	require.NoError(t, err)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 2)
	require.Equal(t, 46.8, *candidates[0].EffectiveCostPrice)
	require.Equal(t, 46.8, *candidates[1].EffectiveCostPrice)
}

func TestSheinCandidateServiceRefreshCandidatesDoesNotShareSDSCostAcrossSourcesWithSameStyleSuffix(t *testing.T) {
	t.Parallel()

	repo := newSheinCandidateRepoStub([]SheinSyncedProductRecord{
		{
			ID:                 311,
			TenantID:           11,
			StoreID:            22,
			SKCName:            "canvas-large",
			SupplierCode:       "MG8006905001-B3195DA6",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(46.8),
			PriceSnapshot:      `{"sale_price":90}`,
			InventorySnapshot:  `{"available":8}`,
			IsActive:           true,
		},
		{
			ID:                 312,
			TenantID:           11,
			StoreID:            22,
			SKCName:            "canvas-small",
			SupplierCode:       "MG8006905002-B3195DA6",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(39.1),
			PriceSnapshot:      `{"sale_price":88}`,
			InventorySnapshot:  `{"available":7}`,
			IsActive:           true,
		},
	})

	service := NewSheinCandidateService(repo)

	_, err := service.RefreshCandidates(context.Background(), 11, 22, "PROMOTION")
	require.NoError(t, err)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 2)
	require.Equal(t, 46.8, *candidates[0].EffectiveCostPrice)
	require.Equal(t, 39.1, *candidates[1].EffectiveCostPrice)
}

func TestSheinCandidateServiceRefreshCandidatesUsesSDSCostGroupOverride(t *testing.T) {
	t.Parallel()

	repo := newSheinCandidateRepoStub([]SheinSyncedProductRecord{
		{
			ID:                 401,
			TenantID:           11,
			StoreID:            22,
			SKCName:            "canvas-large",
			SupplierCode:       "MG8006905001-B3195DA6",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(46.8),
			PriceSnapshot:      `{"sale_price":100}`,
			InventorySnapshot:  `{"available":8}`,
			IsActive:           true,
		},
		{
			ID:                 402,
			TenantID:           11,
			StoreID:            22,
			SKCName:            "canvas-small",
			SupplierCode:       "MG8006905001-C3195DA6",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(39.1),
			PriceSnapshot:      `{"sale_price":80}`,
			InventorySnapshot:  `{"available":7}`,
			IsActive:           true,
		},
	})
	repo.seedSDSCostGroup(SheinSDSCostGroupRecord{
		TenantID:        11,
		StoreID:         22,
		GroupKey:        "style:B3195DA6",
		ManualCostPrice: float64Ptr(50),
	})

	service := NewSheinCandidateService(repo)

	_, err := service.RefreshCandidates(context.Background(), 11, 22, "PROMOTION")
	require.NoError(t, err)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 2)
	require.Equal(t, 50.0, *candidates[0].EffectiveCostPrice)
	require.Equal(t, 50.0, *candidates[1].EffectiveCostPrice)
	require.Equal(t, 0.375, *candidates[1].CalculatedProfitRate)
}

func TestSheinCandidateServiceRefreshCandidatesUsesSourceVariantCostGroupOverride(t *testing.T) {
	t.Parallel()

	product := SheinSyncedProductRecord{
		ID:                 405,
		TenantID:           11,
		StoreID:            22,
		SKCName:            "sh260603194059486654294",
		SupplierCode:       "XB0603003001-181EB5DF",
		SaleName:           "多色",
		ShelfStatus:        "ON_SHELF",
		EffectiveCostPrice: float64Ptr(30),
		PriceSnapshot:      `{"sale_price":100}`,
		InventorySnapshot:  `{"available":8}`,
		SiteSnapshot:       `{"sku_info":[{"sku_code":"sku-color-a-12","supplier_sku":"XB0603003001-V381-TF7E6627E-RB6679CE2-7192C992"},{"sku_code":"sku-color-a-20","supplier_sku":"XB0603003002-V382-TF7E6627E-RB6679CE2-7192C992"}]}`,
		IsActive:           true,
	}
	repo := newSheinCandidateRepoStub([]SheinSyncedProductRecord{
		product,
		{
			ID:                 406,
			TenantID:           11,
			StoreID:            22,
			SKCName:            "sh260529213967065725887",
			SupplierCode:       "XB0603003001-3D8E8A5E",
			SaleName:           "白色",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(30),
			PriceSnapshot:      `{"sale_price":100}`,
			InventorySnapshot:  `{"available":8}`,
			SiteSnapshot:       `{"sku_info":[{"sku_code":"sku-white-12","supplier_sku":"XB0603003001-V381-TF7E6627E-RB6679CE2-7192C992"}]}`,
			IsActive:           true,
		},
	})
	variantIdentity := ResolveSheinSDSVariantCostGroupIdentity(product)
	repo.seedSDSCostGroup(SheinSDSCostGroupRecord{
		TenantID:        11,
		StoreID:         22,
		GroupKey:        variantIdentity.GroupKey,
		ManualCostPrice: float64Ptr(55),
	})

	service := NewSheinCandidateService(repo)

	_, err := service.RefreshCandidates(context.Background(), 11, 22, "PROMOTION")
	require.NoError(t, err)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 2)
	require.Equal(t, 55.0, *candidates[0].EffectiveCostPrice)
	require.Equal(t, 0.45, *candidates[0].CalculatedProfitRate)
	require.Equal(t, 55.0, *candidates[1].EffectiveCostPrice)
	require.Equal(t, 0.45, *candidates[1].CalculatedProfitRate)
}

func TestSheinCandidateServiceRefreshCandidatesFetchesSDSCostGroupsBeyondFirstPage(t *testing.T) {
	t.Parallel()

	products := make([]SheinSyncedProductRecord, 0, 103)
	for i := 0; i < 102; i++ {
		products = append(products, SheinSyncedProductRecord{
			ID:                int64(i + 1),
			TenantID:          11,
			StoreID:           22,
			SKCName:           fmt.Sprintf("filler-%03d", i),
			SupplierCode:      fmt.Sprintf("AA%010d-ABCDEFGH", i),
			ShelfStatus:       "ON_SHELF",
			PriceSnapshot:     `{"sale_price":100}`,
			InventorySnapshot: `{"available":8}`,
			IsActive:          true,
		})
	}
	products = append(products, SheinSyncedProductRecord{
		ID:                200,
		TenantID:          11,
		StoreID:           22,
		SKCName:           "sg260524220749889172313",
		SupplierCode:      "XB0614000001-328D4DCA",
		SaleName:          "White",
		ShelfStatus:       "ON_SHELF",
		PriceSnapshot:     `{"sale_price":61.2}`,
		InventorySnapshot: `{"available":999}`,
		SiteSnapshot:      `{"sku_codes":["I3MPJUQZ9KCJBH"],"sku_info":[{"sku_code":"I3mpjuqz9kcjbh","variant_label":"均码"}]}`,
		IsActive:          true,
	})
	repo := newSheinCandidateRepoStub(products)
	for i := 0; i < 102; i++ {
		repo.seedSDSCostGroup(SheinSDSCostGroupRecord{
			TenantID:        11,
			StoreID:         22,
			GroupKey:        fmt.Sprintf("source:AA%010d", i),
			ManualCostPrice: float64Ptr(10),
		})
	}
	repo.seedSDSCostGroup(SheinSDSCostGroupRecord{
		TenantID:        11,
		StoreID:         22,
		GroupKey:        "source:XB0614000001:variant:0DDA0C15301B",
		ManualCostPrice: float64Ptr(23.88),
	})

	service := NewSheinCandidateService(repo)

	_, err := service.RefreshCandidates(context.Background(), 11, 22, "PROMOTION")
	require.NoError(t, err)

	var target SheinActivityCandidateRecord
	for _, candidate := range repo.savedCandidates() {
		if candidate.SKCName == "sg260524220749889172313" {
			target = candidate
			break
		}
	}
	require.NotNil(t, target.EffectiveCostPrice)
	require.Equal(t, 23.88, *target.EffectiveCostPrice)
	require.Equal(t, SheinCandidateEligibilityStatusEligible, target.EligibilityStatus)
	require.Greater(t, len(repo.sdsGroupQueries), 1)
}

func TestSheinCandidateServiceRefreshCandidatesMarksNonOnShelfRowsIneligibleAndIgnoresInactiveRows(t *testing.T) {
	t.Parallel()

	repo := newSheinCandidateRepoStub([]SheinSyncedProductRecord{
		{
			ID:                 21,
			TenantID:           7,
			StoreID:            8,
			SKCName:            "off-shelf-skc",
			ShelfStatus:        "OFF_SHELF",
			EffectiveCostPrice: float64Ptr(18.8),
			IsActive:           true,
		},
		{
			ID:                 22,
			TenantID:           7,
			StoreID:            8,
			SKCName:            "inactive-skc",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(18.8),
			IsActive:           false,
		},
	})

	service := NewSheinCandidateService(repo)

	result, err := service.RefreshCandidates(context.Background(), 7, 8, "flash_sale")
	require.NoError(t, err)
	require.Equal(t, 0, result.EligibleCount)
	require.Equal(t, 1, result.TotalCount)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 1)
	require.Equal(t, "off-shelf-skc", candidates[0].SKCName)
	require.Equal(t, SheinCandidateEligibilityStatusIneligible, candidates[0].EligibilityStatus)
	require.NotEmpty(t, candidates[0].EligibilityReason)
}

func TestSheinCandidateServiceRefreshCandidatesPreservesWorkflowStateForSameVersion(t *testing.T) {
	t.Parallel()

	repo := newSheinCandidateRepoStub([]SheinSyncedProductRecord{
		{
			ID:                 31,
			TenantID:           9,
			StoreID:            10,
			SKCName:            "same-version-skc",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(22.2),
			PriceSnapshot:      `{"sale_price":49.9}`,
			InventorySnapshot:  `{"available":50}`,
			SyncVersion:        "sync-v1",
			IsActive:           true,
		},
	})
	repo.seedCandidate(SheinActivityCandidateRecord{
		ID:                 1001,
		TenantID:           9,
		StoreID:            10,
		SyncedProductID:    31,
		ActivityType:       "flash_sale",
		ActivityKey:        "flash_sale:9:10",
		SKCName:            "same-version-skc",
		CandidateVersion:   buildSheinCandidateVersion(repo.products[0]),
		EligibilityStatus:  SheinCandidateEligibilityStatusEligible,
		ReviewStatus:       SheinCandidateReviewStatusApproved,
		AutoModeEligible:   true,
		SelectedForRun:     true,
		EffectiveCostPrice: float64Ptr(22.2),
	})

	service := NewSheinCandidateService(repo)

	result, err := service.RefreshCandidates(context.Background(), 9, 10, "flash_sale")
	require.NoError(t, err)
	require.Equal(t, 1, result.EligibleCount)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 1)
	require.Equal(t, SheinCandidateReviewStatusApproved, candidates[0].ReviewStatus)
	require.True(t, candidates[0].AutoModeEligible)
	require.True(t, candidates[0].SelectedForRun)
}

func TestSheinCandidateServiceRefreshCandidatesTreatsSKUCostChangesAsNewVersion(t *testing.T) {
	t.Parallel()

	oldProduct := SheinSyncedProductRecord{
		ID:                 32,
		TenantID:           9,
		StoreID:            10,
		SKCName:            "sku-cost-version-skc",
		ShelfStatus:        "ON_SHELF",
		EffectiveCostPrice: float64Ptr(22.2),
		PriceSnapshot:      `{"sale_price":49.9}`,
		InventorySnapshot:  `{"available":50}`,
		SyncVersion:        "sync-v1",
		IsActive:           true,
		SKUCostPriceInfoList: []SheinSKUCostPrice{
			{SKUCode: "SKU-A", CostPrice: 18.8},
			{SKUCode: "SKU-B", CostPrice: 22.2},
		},
	}
	newProduct := oldProduct
	newProduct.SKUCostPriceInfoList = []SheinSKUCostPrice{
		{SKUCode: "SKU-A", CostPrice: 19.9},
		{SKUCode: "SKU-B", CostPrice: 22.2},
	}

	repo := newSheinCandidateRepoStub([]SheinSyncedProductRecord{newProduct})
	repo.seedCandidate(SheinActivityCandidateRecord{
		ID:                   1002,
		TenantID:             9,
		StoreID:              10,
		SyncedProductID:      32,
		ActivityType:         "flash_sale",
		ActivityKey:          "flash_sale:9:10",
		SKCName:              "sku-cost-version-skc",
		CandidateVersion:     buildSheinCandidateVersion(oldProduct),
		EligibilityStatus:    SheinCandidateEligibilityStatusEligible,
		ReviewStatus:         SheinCandidateReviewStatusApproved,
		AutoModeEligible:     true,
		SelectedForRun:       true,
		EffectiveCostPrice:   float64Ptr(22.2),
		SKUCostPriceInfoList: oldProduct.SKUCostPriceInfoList,
	})

	service := NewSheinCandidateService(repo)

	result, err := service.RefreshCandidates(context.Background(), 9, 10, "flash_sale")
	require.NoError(t, err)
	require.Equal(t, 1, result.EligibleCount)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 2)

	byVersion := make(map[string]SheinActivityCandidateRecord, len(candidates))
	for _, candidate := range candidates {
		byVersion[candidate.CandidateVersion] = candidate
	}
	latest := byVersion[buildSheinCandidateVersion(newProduct)]
	stale := byVersion[buildSheinCandidateVersion(oldProduct)]
	require.NotEmpty(t, latest.CandidateVersion)
	require.NotEmpty(t, stale.CandidateVersion)
	require.Equal(t, SheinCandidateReviewStatusPendingReview, latest.ReviewStatus)
	require.False(t, latest.AutoModeEligible)
	require.False(t, latest.SelectedForRun)
	require.Equal(t, newProduct.SKUCostPriceInfoList, latest.SKUCostPriceInfoList)
	require.Equal(t, SheinCandidateEligibilityStatusIneligible, stale.EligibilityStatus)
	require.Equal(t, SheinCandidateReviewStatusRejected, stale.ReviewStatus)
	require.False(t, stale.AutoModeEligible)
	require.False(t, stale.SelectedForRun)
}

func TestSheinCandidateServiceRefreshCandidatesSupersedesOlderCandidateVersions(t *testing.T) {
	t.Parallel()

	oldProduct := SheinSyncedProductRecord{
		ID:                 41,
		TenantID:           12,
		StoreID:            13,
		SKCName:            "versioned-skc",
		ShelfStatus:        "ON_SHELF",
		EffectiveCostPrice: float64Ptr(18.8),
		PriceSnapshot:      `{"sale_price":39.9}`,
		InventorySnapshot:  `{"available":12}`,
		SyncVersion:        "sync-v1",
		IsActive:           true,
	}
	newProduct := oldProduct
	newProduct.PriceSnapshot = `{"sale_price":35.9}`

	repo := newSheinCandidateRepoStub([]SheinSyncedProductRecord{newProduct})
	repo.seedCandidate(SheinActivityCandidateRecord{
		ID:                 2001,
		TenantID:           12,
		StoreID:            13,
		SyncedProductID:    41,
		ActivityType:       "flash_sale",
		ActivityKey:        "flash_sale:12:13",
		SKCName:            "versioned-skc",
		CandidateVersion:   buildSheinCandidateVersion(oldProduct),
		EligibilityStatus:  SheinCandidateEligibilityStatusEligible,
		ReviewStatus:       SheinCandidateReviewStatusApproved,
		AutoModeEligible:   true,
		SelectedForRun:     true,
		EffectiveCostPrice: float64Ptr(18.8),
	})

	service := NewSheinCandidateService(repo)

	result, err := service.RefreshCandidates(context.Background(), 12, 13, "flash_sale")
	require.NoError(t, err)
	require.Equal(t, 1, result.EligibleCount)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 2)

	var latest, stale *SheinActivityCandidateRecord
	for i := range candidates {
		candidate := candidates[i]
		if candidate.CandidateVersion == buildSheinCandidateVersion(newProduct) {
			latest = &candidate
			continue
		}
		if candidate.CandidateVersion == buildSheinCandidateVersion(oldProduct) {
			stale = &candidate
		}
	}

	require.NotNil(t, latest)
	require.NotNil(t, stale)
	require.Equal(t, SheinCandidateReviewStatusPendingReview, latest.ReviewStatus)
	require.False(t, latest.AutoModeEligible)
	require.False(t, latest.SelectedForRun)
	require.Equal(t, SheinCandidateEligibilityStatusIneligible, stale.EligibilityStatus)
	require.Equal(t, SheinCandidateReviewStatusRejected, stale.ReviewStatus)
	require.False(t, stale.AutoModeEligible)
	require.False(t, stale.SelectedForRun)
	require.Contains(t, stale.EligibilityReason, "superseded")
}

func TestSheinCandidateServiceRefreshCandidatesSupersedesCandidatesForMissingSKC(t *testing.T) {
	t.Parallel()

	repo := newSheinCandidateRepoStub([]SheinSyncedProductRecord{
		{
			ID:                 51,
			TenantID:           21,
			StoreID:            22,
			SKCName:            "still-active-skc",
			ShelfStatus:        "ON_SHELF",
			EffectiveCostPrice: float64Ptr(16.6),
			PriceSnapshot:      `{"sale_price":29.9}`,
			InventorySnapshot:  `{"available":11}`,
			SyncVersion:        "sync-v2",
			IsActive:           true,
		},
	})
	repo.seedCandidate(SheinActivityCandidateRecord{
		ID:                 3001,
		TenantID:           21,
		StoreID:            22,
		SyncedProductID:    99,
		ActivityType:       "flash_sale",
		ActivityKey:        "flash_sale:21:22",
		SKCName:            "missing-skc",
		CandidateVersion:   "old-version",
		EligibilityStatus:  SheinCandidateEligibilityStatusEligible,
		ReviewStatus:       SheinCandidateReviewStatusApproved,
		AutoModeEligible:   true,
		SelectedForRun:     true,
		EffectiveCostPrice: float64Ptr(19.9),
	})

	service := NewSheinCandidateService(repo)

	result, err := service.RefreshCandidates(context.Background(), 21, 22, "flash_sale")
	require.NoError(t, err)
	require.Equal(t, 1, result.EligibleCount)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 2)

	var missing, active *SheinActivityCandidateRecord
	for i := range candidates {
		candidate := candidates[i]
		if candidate.SKCName == "missing-skc" {
			missing = &candidate
			continue
		}
		if candidate.SKCName == "still-active-skc" {
			active = &candidate
		}
	}

	require.NotNil(t, active)
	require.NotNil(t, missing)
	require.Equal(t, SheinCandidateEligibilityStatusEligible, active.EligibilityStatus)
	require.Equal(t, SheinCandidateEligibilityStatusIneligible, missing.EligibilityStatus)
	require.Equal(t, SheinCandidateReviewStatusRejected, missing.ReviewStatus)
	require.False(t, missing.AutoModeEligible)
	require.False(t, missing.SelectedForRun)
	require.Contains(t, missing.EligibilityReason, "superseded")
}

func TestSheinCandidateServiceResetCandidatesResetsMatchingCandidates(t *testing.T) {
	t.Parallel()

	repo := newSheinCandidateRepoStub(nil)
	service := NewSheinCandidateService(repo)
	activityKey := buildSheinActivityKey("TIME_LIMITED", 227, 870)

	repo.seedCandidate(SheinActivityCandidateRecord{
		ID:                1,
		TenantID:          227,
		StoreID:           870,
		ActivityType:      "TIME_LIMITED",
		ActivityKey:       activityKey,
		SKCName:           "skc-missing-cost",
		CandidateVersion:  "v1",
		EligibilityStatus: SheinCandidateEligibilityStatusIneligible,
		EligibilityReason: "missing effective cost price",
		ReviewStatus:      SheinCandidateReviewStatusFailed,
		AutoModeEligible:  true,
		SelectedForRun:    true,
	})
	repo.seedCandidate(SheinActivityCandidateRecord{
		ID:                2,
		TenantID:          227,
		StoreID:           870,
		ActivityType:      "TIME_LIMITED",
		ActivityKey:       activityKey,
		SKCName:           "skc-other-reason",
		CandidateVersion:  "v1",
		EligibilityStatus: SheinCandidateEligibilityStatusIneligible,
		EligibilityReason: "product is not on shelf",
		ReviewStatus:      SheinCandidateReviewStatusFailed,
		AutoModeEligible:  true,
		SelectedForRun:    true,
	})
	repo.seedCandidate(SheinActivityCandidateRecord{
		ID:                3,
		TenantID:          227,
		StoreID:           870,
		ActivityType:      "TIME_LIMITED",
		ActivityKey:       activityKey,
		SKCName:           "skc-enrolled",
		CandidateVersion:  "v1",
		EligibilityStatus: SheinCandidateEligibilityStatusIneligible,
		EligibilityReason: "missing effective cost price",
		ReviewStatus:      SheinCandidateReviewStatusEnrolled,
		AutoModeEligible:  true,
		SelectedForRun:    true,
	})

	result, err := service.ResetCandidates(context.Background(), 227, 870, SheinCandidateResetRequest{
		ActivityType:      "TIME_LIMITED",
		EligibilityReason: "missing effective cost price",
	})

	require.NoError(t, err)
	require.Equal(t, 2, result.MatchedCount)
	require.Equal(t, 2, result.ResetCount)
	require.Equal(t, 0, result.SkippedCount)

	rows, _, err := repo.ListCandidates(context.Background(), &SheinActivityCandidateQuery{
		TenantID:     227,
		StoreID:      870,
		ActivityType: "TIME_LIMITED",
		PageSize:     10,
	})
	require.NoError(t, err)
	byID := map[int64]SheinActivityCandidateRecord{}
	for _, row := range rows {
		byID[row.ID] = row
	}

	reset := byID[1]
	require.Equal(t, SheinCandidateReviewStatusPendingReview, reset.ReviewStatus)
	require.False(t, reset.AutoModeEligible)
	require.False(t, reset.SelectedForRun)
	require.Equal(t, SheinCandidateEligibilityStatusIneligible, reset.EligibilityStatus)
	require.Equal(t, "missing effective cost price", reset.EligibilityReason)

	unchangedReason := byID[2]
	require.Equal(t, SheinCandidateReviewStatusFailed, unchangedReason.ReviewStatus)
	require.True(t, unchangedReason.AutoModeEligible)
	require.True(t, unchangedReason.SelectedForRun)

	resetEnrolled := byID[3]
	require.Equal(t, SheinCandidateReviewStatusPendingReview, resetEnrolled.ReviewStatus)
	require.False(t, resetEnrolled.AutoModeEligible)
	require.False(t, resetEnrolled.SelectedForRun)
}

func TestSheinCandidateServiceResetCandidatesSkipsRunningEnrollmentItems(t *testing.T) {
	t.Parallel()

	repo := newSheinCandidateRepoStub(nil)
	service := NewSheinCandidateService(repo)
	activityKey := buildSheinActivityKey("TIME_LIMITED", 227, 870)

	repo.seedCandidate(SheinActivityCandidateRecord{
		ID:                11,
		TenantID:          227,
		StoreID:           870,
		ActivityType:      "TIME_LIMITED",
		ActivityKey:       activityKey,
		SKCName:           "skc-running",
		CandidateVersion:  "v1",
		EligibilityStatus: SheinCandidateEligibilityStatusEligible,
		ReviewStatus:      SheinCandidateReviewStatusFailed,
		AutoModeEligible:  true,
		SelectedForRun:    true,
	})
	repo.seedCandidate(SheinActivityCandidateRecord{
		ID:                12,
		TenantID:          227,
		StoreID:           870,
		ActivityType:      "TIME_LIMITED",
		ActivityKey:       activityKey,
		SKCName:           "skc-resettable",
		CandidateVersion:  "v1",
		EligibilityStatus: SheinCandidateEligibilityStatusEligible,
		ReviewStatus:      SheinCandidateReviewStatusFailed,
		AutoModeEligible:  true,
		SelectedForRun:    true,
	})
	repo.seedEnrollmentItem(SheinActivityEnrollmentItemRecord{
		RunID:       101,
		CandidateID: 11,
		StoreID:     870,
		ActivityKey: activityKey,
		Status:      SheinEnrollmentItemStatusRunning,
	})

	result, err := service.ResetCandidates(context.Background(), 227, 870, SheinCandidateResetRequest{
		ActivityType: "TIME_LIMITED",
		CandidateIDs: []int64{11, 12},
	})

	require.NoError(t, err)
	require.Equal(t, 2, result.MatchedCount)
	require.Equal(t, 1, result.ResetCount)
	require.Equal(t, 1, result.SkippedCount)

	rows, _, err := repo.ListCandidates(context.Background(), &SheinActivityCandidateQuery{
		TenantID:     227,
		StoreID:      870,
		ActivityType: "TIME_LIMITED",
		CandidateIDs: []int64{11, 12},
		PageSize:     2,
	})
	require.NoError(t, err)
	byID := map[int64]SheinActivityCandidateRecord{}
	for _, row := range rows {
		byID[row.ID] = row
	}

	running := byID[11]
	require.Equal(t, SheinCandidateReviewStatusFailed, running.ReviewStatus)
	require.True(t, running.AutoModeEligible)
	require.True(t, running.SelectedForRun)

	resettable := byID[12]
	require.Equal(t, SheinCandidateReviewStatusPendingReview, resettable.ReviewStatus)
	require.False(t, resettable.AutoModeEligible)
	require.False(t, resettable.SelectedForRun)
}

type sheinCandidateRepoStub struct {
	mu              sync.RWMutex
	products        []SheinSyncedProductRecord
	queries         []*SheinSyncedProductQuery
	sdsGroupQueries []*SheinSDSCostGroupQuery
	candidates      map[string]SheinActivityCandidateRecord
	enrollmentItems []SheinActivityEnrollmentItemRecord
	sdsGroups       map[string]SheinSDSCostGroupRecord
}

func newSheinCandidateRepoStub(products []SheinSyncedProductRecord) *sheinCandidateRepoStub {
	cloned := make([]SheinSyncedProductRecord, 0, len(products))
	for _, product := range products {
		cloned = append(cloned, cloneSheinCandidateTestProduct(product))
	}
	return &sheinCandidateRepoStub{
		products:   cloned,
		candidates: make(map[string]SheinActivityCandidateRecord),
		sdsGroups:  make(map[string]SheinSDSCostGroupRecord),
	}
}

func (r *sheinCandidateRepoStub) ListSyncedProducts(_ context.Context, query *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.queries = append(r.queries, cloneSheinCandidateQuery(query))

	items := make([]SheinSyncedProductRecord, 0, len(r.products))
	for _, row := range r.products {
		if query != nil {
			if query.TenantID > 0 && row.TenantID != query.TenantID {
				continue
			}
			if query.StoreID > 0 && row.StoreID != query.StoreID {
				continue
			}
			if query.IsActive != nil && row.IsActive != *query.IsActive {
				continue
			}
		}
		items = append(items, cloneSheinCandidateTestProduct(row))
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].SKCName < items[j].SKCName
	})

	total := int64(len(items))
	page, pageSize := 1, len(items)
	if query != nil {
		if query.Page > 0 {
			page = query.Page
		}
		if query.PageSize > 0 {
			pageSize = query.PageSize
		}
	}
	if pageSize == 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []SheinSyncedProductRecord{}, total, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total, nil
}

func (r *sheinCandidateRepoStub) SaveCandidates(_ context.Context, records []*SheinActivityCandidateRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, record := range records {
		if record == nil {
			continue
		}
		key := fmt.Sprintf("%d|%d|%s|%s|%s|%s", record.TenantID, record.StoreID, record.ActivityType, record.ActivityKey, record.SKCName, record.CandidateVersion)
		r.candidates[key] = cloneSheinCandidateTestCandidate(*record)
	}
	return nil
}

func (r *sheinCandidateRepoStub) ListCandidates(_ context.Context, query *SheinActivityCandidateQuery) ([]SheinActivityCandidateRecord, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]SheinActivityCandidateRecord, 0, len(r.candidates))
	for _, row := range r.candidates {
		if !matchesSheinCandidateTestQuery(row, query) {
			continue
		}
		items = append(items, cloneSheinCandidateTestCandidate(row))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID != items[j].ID {
			return items[i].ID < items[j].ID
		}
		if items[i].SKCName != items[j].SKCName {
			return items[i].SKCName < items[j].SKCName
		}
		return items[i].CandidateVersion < items[j].CandidateVersion
	})

	total := int64(len(items))
	page, pageSize := 1, len(items)
	if query != nil {
		if query.Page > 0 {
			page = query.Page
		}
		if query.PageSize > 0 {
			pageSize = query.PageSize
		}
	}
	if pageSize == 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []SheinActivityCandidateRecord{}, total, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total, nil
}

func (r *sheinCandidateRepoStub) ListEnrollmentItems(_ context.Context, query *SheinEnrollmentItemQuery) ([]SheinActivityEnrollmentItemRecord, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]SheinActivityEnrollmentItemRecord, 0, len(r.enrollmentItems))
	for _, row := range r.enrollmentItems {
		if query != nil {
			if query.StoreID > 0 && row.StoreID != query.StoreID {
				continue
			}
			if query.Status != nil && row.Status != *query.Status {
				continue
			}
			if len(query.CandidateIDs) > 0 && !containsSheinCandidateID(query.CandidateIDs, row.CandidateID) {
				continue
			}
		}
		items = append(items, row)
	}
	return items, int64(len(items)), nil
}

func (r *sheinCandidateRepoStub) ListSDSCostGroups(_ context.Context, query *SheinSDSCostGroupQuery) ([]SheinSDSCostGroupRecord, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.sdsGroupQueries = append(r.sdsGroupQueries, cloneSheinCandidateSDSCostGroupQuery(query))
	items := make([]SheinSDSCostGroupRecord, 0, len(r.sdsGroups))
	for _, row := range r.sdsGroups {
		if query != nil {
			if query.TenantID > 0 && row.TenantID != query.TenantID {
				continue
			}
			if query.StoreID > 0 && row.StoreID != query.StoreID {
				continue
			}
			if len(query.GroupKeys) > 0 && !containsSheinCandidateGroupKey(query.GroupKeys, row.GroupKey) {
				continue
			}
		}
		items = append(items, row)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].GroupKey < items[j].GroupKey
	})
	total := int64(len(items))
	page, pageSize := 1, len(items)
	if query != nil {
		if query.Page > 0 {
			page = query.Page
		}
		if query.PageSize > 0 {
			pageSize = query.PageSize
		}
	}
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []SheinSDSCostGroupRecord{}, total, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total, nil
}

func (r *sheinCandidateRepoStub) savedCandidates() []SheinActivityCandidateRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]SheinActivityCandidateRecord, 0, len(r.candidates))
	for _, row := range r.candidates {
		items = append(items, cloneSheinCandidateTestCandidate(row))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].SKCName < items[j].SKCName
	})
	return items
}

func (r *sheinCandidateRepoStub) seedSDSCostGroup(record SheinSDSCostGroupRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sdsGroups[record.GroupKey] = record
}

func (r *sheinCandidateRepoStub) seedCandidate(record SheinActivityCandidateRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%d|%d|%s|%s|%s|%s", record.TenantID, record.StoreID, record.ActivityType, record.ActivityKey, record.SKCName, record.CandidateVersion)
	r.candidates[key] = cloneSheinCandidateTestCandidate(record)
}

func (r *sheinCandidateRepoStub) seedEnrollmentItem(record SheinActivityEnrollmentItemRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.enrollmentItems = append(r.enrollmentItems, record)
}

func containsSheinCandidateID(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsSheinCandidateGroupKey(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func cloneSheinCandidateTestProduct(row SheinSyncedProductRecord) SheinSyncedProductRecord {
	row.AutoCostPrice = cloneServiceTestFloat64(row.AutoCostPrice)
	row.ManualCostPrice = cloneServiceTestFloat64(row.ManualCostPrice)
	row.EffectiveCostPrice = cloneServiceTestFloat64(row.EffectiveCostPrice)
	row.PublishTime = cloneServiceTestTime(row.PublishTime)
	row.FirstShelfTime = cloneServiceTestTime(row.FirstShelfTime)
	row.LastSyncAt = cloneServiceTestTime(row.LastSyncAt)
	row.SKUCostPriceInfoList = cloneSheinSKUCostPriceList(row.SKUCostPriceInfoList)
	return row
}

func cloneSheinCandidateTestCandidate(row SheinActivityCandidateRecord) SheinActivityCandidateRecord {
	row.EffectiveCostPrice = cloneServiceTestFloat64(row.EffectiveCostPrice)
	row.CalculatedProfitRate = cloneServiceTestFloat64(row.CalculatedProfitRate)
	row.SKUCostPriceInfoList = cloneSheinSKUCostPriceList(row.SKUCostPriceInfoList)
	return row
}

func cloneSheinCandidateQuery(query *SheinSyncedProductQuery) *SheinSyncedProductQuery {
	if query == nil {
		return nil
	}
	cloned := *query
	if query.IsActive != nil {
		active := *query.IsActive
		cloned.IsActive = &active
	}
	return &cloned
}

func cloneSheinCandidateSDSCostGroupQuery(query *SheinSDSCostGroupQuery) *SheinSDSCostGroupQuery {
	if query == nil {
		return nil
	}
	cloned := *query
	cloned.GroupKeys = append([]string(nil), query.GroupKeys...)
	return &cloned
}

func matchesSheinCandidateTestQuery(row SheinActivityCandidateRecord, query *SheinActivityCandidateQuery) bool {
	if query == nil {
		return true
	}
	if query.TenantID > 0 && row.TenantID != query.TenantID {
		return false
	}
	if query.StoreID > 0 && row.StoreID != query.StoreID {
		return false
	}
	if query.ActivityType != "" && row.ActivityType != query.ActivityType {
		return false
	}
	if query.ActivityKey != "" && row.ActivityKey != query.ActivityKey {
		return false
	}
	if query.SKCName != "" && row.SKCName != query.SKCName {
		return false
	}
	return true
}
