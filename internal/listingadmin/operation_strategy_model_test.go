package listingadmin

import "testing"

func TestApplyOperationStrategyAuditFieldsSetsOwnerAndAuditColumns(t *testing.T) {
	t.Parallel()

	row := listingOperationStrategy{}
	applyOperationStrategyAuditFields(&row, "user-1", true)

	if row.OwnerUserID != "user-1" {
		t.Fatalf("ownerUserID = %q, want user-1", row.OwnerUserID)
	}
	if row.Creator != "user-1" || row.CreatedBy != "user-1" {
		t.Fatalf("creator/createdBy = %q/%q, want user-1/user-1", row.Creator, row.CreatedBy)
	}
	if row.Updater != "user-1" || row.UpdatedBy != "user-1" {
		t.Fatalf("updater/updatedBy = %q/%q, want user-1/user-1", row.Updater, row.UpdatedBy)
	}
}

func TestListingOperationStrategyConversionPreservesOptionalFields(t *testing.T) {
	t.Parallel()

	stockThreshold := 8
	minProfit := 0.15
	priceMultiplier := 1.2
	fixedAdjustment := 2.5
	stockRatio := 0.6
	strategy := OperationStrategy{
		ID:                    1,
		TenantID:              101,
		StoreID:               201,
		Name:                  " Daily ops ",
		Platform:              " SHEIN ",
		Status:                2,
		StockChangeThreshold:  &stockThreshold,
		StockChangeAction:     " pause ",
		OutOfStockAction:      " delist ",
		MinProfitRate:         &minProfit,
		LowProfitAction:       " keep ",
		PriceUpdateMultiplier: &priceMultiplier,
		FixedPriceAdjustment:  &fixedAdjustment,
		StockUpdateRatio:      &stockRatio,
		Remark:                " note ",
	}

	row := listingOperationStrategyFromOperationStrategy(&strategy)
	if row.Name != "Daily ops" || row.Platform != "SHEIN" || row.StockChangeAction != "pause" {
		t.Fatalf("trimmed row = %+v, want trimmed strings", row)
	}
	if row.StockChangeThreshold != stockThreshold || row.MinProfitRate != minProfit || row.StockUpdateRatio != stockRatio {
		t.Fatalf("row numeric optionals = %+v, want preserved numeric values", row)
	}

	converted := row.toOperationStrategy()
	if converted.StockChangeThreshold == nil || *converted.StockChangeThreshold != stockThreshold {
		t.Fatalf("converted stockChangeThreshold = %v, want %d", converted.StockChangeThreshold, stockThreshold)
	}
	if converted.MinProfitRate == nil || *converted.MinProfitRate != minProfit {
		t.Fatalf("converted minProfitRate = %v, want %v", converted.MinProfitRate, minProfit)
	}
	if converted.PriceUpdateMultiplier == nil || *converted.PriceUpdateMultiplier != priceMultiplier {
		t.Fatalf("converted priceUpdateMultiplier = %v, want %v", converted.PriceUpdateMultiplier, priceMultiplier)
	}
	if converted.FixedPriceAdjustment == nil || *converted.FixedPriceAdjustment != fixedAdjustment {
		t.Fatalf("converted fixedPriceAdjustment = %v, want %v", converted.FixedPriceAdjustment, fixedAdjustment)
	}
	if converted.StockUpdateRatio == nil || *converted.StockUpdateRatio != stockRatio {
		t.Fatalf("converted stockUpdateRatio = %v, want %v", converted.StockUpdateRatio, stockRatio)
	}
	if converted.Remark != "note" || converted.LowProfitAction != "keep" {
		t.Fatalf("converted = %+v, want trimmed values preserved", converted)
	}
}
