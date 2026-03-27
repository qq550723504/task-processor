package pricing_test

import (
	"testing"

	"task-processor/internal/shein/api/marketing"
	"task-processor/internal/shein/pricing"
)

func TestCostProfitCalculator_CalculateCostAndProfit(t *testing.T) {
	calc := pricing.NewCostProfitCalculator()

	tests := []struct {
		name           string
		product        marketing.SkcInfo
		wantCostPrice  float64
		wantProfitRate float64
	}{
		{
			name: "normal_with_profit",
			product: marketing.SkcInfo{
				SupplyPrice: 100,
				SitePriceInfoList: []marketing.SitePriceInfo{
					{IsAvailable: true, SalePrice: 200},
					{IsAvailable: true, SalePrice: 300},
				},
			},
			wantCostPrice:  85,                              // 100 * 0.85
			wantProfitRate: ((250.0 - 85.0) / 85.0) * 100.0, // avgSale=250
		},
		{
			name: "zero_supply_price",
			product: marketing.SkcInfo{
				SupplyPrice: 0,
				SitePriceInfoList: []marketing.SitePriceInfo{
					{IsAvailable: true, SalePrice: 100},
				},
			},
			wantCostPrice:  0,
			wantProfitRate: 0, // costPrice=0，不计算利润率
		},
		{
			name: "no_site_prices",
			product: marketing.SkcInfo{
				SupplyPrice:       100,
				SitePriceInfoList: nil,
			},
			wantCostPrice:  85,
			wantProfitRate: 0, // avgSalePrice=0，不满足 avgSalePrice > costPrice
		},
		{
			name: "sale_price_below_cost",
			product: marketing.SkcInfo{
				SupplyPrice: 100,
				SitePriceInfoList: []marketing.SitePriceInfo{
					{IsAvailable: true, SalePrice: 50},
				},
			},
			wantCostPrice:  85,
			wantProfitRate: 0, // avgSalePrice(50) < costPrice(85)
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotCost, gotProfit := calc.CalculateCostAndProfit(tc.product)
			if gotCost != tc.wantCostPrice {
				t.Errorf("costPrice = %v, want %v", gotCost, tc.wantCostPrice)
			}
			if gotProfit != tc.wantProfitRate {
				t.Errorf("profitRate = %v, want %v", gotProfit, tc.wantProfitRate)
			}
		})
	}
}

func TestCostProfitCalculator_CalculateAverageSalePrice(t *testing.T) {
	calc := pricing.NewCostProfitCalculator()

	tests := []struct {
		name    string
		product marketing.SkcInfo
		want    float64
	}{
		{
			name: "all_available",
			product: marketing.SkcInfo{
				SupplyPrice: 10,
				SitePriceInfoList: []marketing.SitePriceInfo{
					{IsAvailable: true, SalePrice: 100},
					{IsAvailable: true, SalePrice: 200},
				},
			},
			want: 150, // (100+200)/2
		},
		{
			name: "none_available",
			product: marketing.SkcInfo{
				SupplyPrice: 10,
				SitePriceInfoList: []marketing.SitePriceInfo{
					{IsAvailable: false, SalePrice: 100},
					{IsAvailable: false, SalePrice: 200},
				},
			},
			want: 0,
		},
		{
			name: "mixed_availability",
			product: marketing.SkcInfo{
				SupplyPrice: 10,
				SitePriceInfoList: []marketing.SitePriceInfo{
					{IsAvailable: true, SalePrice: 300},
					{IsAvailable: false, SalePrice: 100},
					{IsAvailable: true, SalePrice: 100},
				},
			},
			want: 200, // (300+100)/2
		},
		{
			name: "empty_list",
			product: marketing.SkcInfo{
				SupplyPrice:       10,
				SitePriceInfoList: []marketing.SitePriceInfo{},
			},
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// 通过 CalculateCostAndProfit 间接验证 calculateAverageSalePrice
			// 当 supplyPrice=10 时 costPrice=8.5，avgSalePrice > 8.5 时才有利润率
			// 这里直接用一个足够低的 supplyPrice 来让利润率反映 avgSalePrice
			// 但由于 calculateAverageSalePrice 是私有方法，通过公开接口间接测试
			gotCost, gotProfit := calc.CalculateCostAndProfit(tc.product)
			costPrice := tc.product.SupplyPrice * 0.85
			if gotCost != costPrice {
				t.Errorf("costPrice = %v, want %v", gotCost, costPrice)
			}
			if tc.want == 0 {
				if gotProfit != 0 {
					t.Errorf("profitRate = %v, want 0 (avgSalePrice=%v)", gotProfit, tc.want)
				}
			} else if tc.want > costPrice {
				expectedProfit := ((tc.want - costPrice) / costPrice) * 100
				if gotProfit != expectedProfit {
					t.Errorf("profitRate = %v, want %v", gotProfit, expectedProfit)
				}
			}
		})
	}
}
