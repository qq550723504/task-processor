package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPendingPriceListRequest(t *testing.T) {
	req := &PendingPriceListRequest{
		PageSize: 25,
		PageNo:   1,
		Scene:    "PRICING_HEALTH_SALES_BOOST",
	}

	assert.Equal(t, 25, req.PageSize)
	assert.Equal(t, 1, req.PageNo)
	assert.Equal(t, "PRICING_HEALTH_SALES_BOOST", req.Scene)
}

func TestRejectPriceRequest(t *testing.T) {
	req := &RejectPriceRequest{
		GoodsID:         "602906963083875",
		SkuIDs:          []string{"42895046380613"},
		OperationSource: 1005,
	}

	assert.Equal(t, "602906963083875", req.GoodsID)
	assert.Equal(t, 1, len(req.SkuIDs))
	assert.Equal(t, 1005, req.OperationSource)
}

func TestReappealPriceRequest(t *testing.T) {
	req := &ReappealPriceRequest{
		GoodsID:              "602408746850061",
		AppealSource:         100,
		MerchantAppealReason: []string{"LOWER_THAN_SIMILAR"},
		SkuInfoList: []ReappealSkuInfo{
			{
				SkuID:                       "37921474236098",
				SupplierPriceStr:            "213.96",
				RecommendedSupplierPriceStr: "10.60",
				TargetSupplierPriceStr:      "100.00",
				Currency:                    "USD",
			},
		},
	}

	assert.Equal(t, "602408746850061", req.GoodsID)
	assert.Equal(t, 100, req.AppealSource)
	assert.Equal(t, 1, len(req.SkuInfoList))
	assert.Equal(t, "100.00", req.SkuInfoList[0].TargetSupplierPriceStr)
}

func TestAcceptPriceRequest(t *testing.T) {
	req := &AcceptPriceRequest{
		Scene:   2,
		GoodsID: "602204735908247",
		SkuList: []AcceptPriceSkuInfo{
			{
				SkuID:                  "41735405200193",
				Currency:               "USD",
				TargetSupplierPriceStr: "55.67",
			},
		},
	}

	assert.Equal(t, 2, req.Scene)
	assert.Equal(t, "602204735908247", req.GoodsID)
	assert.Equal(t, 1, len(req.SkuList))
	assert.Equal(t, "55.67", req.SkuList[0].TargetSupplierPriceStr)
}
