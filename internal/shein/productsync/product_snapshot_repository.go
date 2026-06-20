// Package productsync 提供 SHEIN 产品快照到 listingadmin 仓储模型的转换
package productsync

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/listingadmin"
	"task-processor/internal/pkg/types"
)

func sheinProductDataFromSnapshot(prod *ProductSnapshot) listingadmin.ProductData {
	item := listingadmin.ProductData{
		TenantID:          prod.TenantID,
		StoreID:           sheinPtrInt64(prod.StoreID),
		Platform:          prod.Platform,
		Region:            prod.Region,
		ParentProductID:   prod.ParentProductID,
		ProductID:         prod.ProductID,
		Title:             prod.Title,
		Description:       prod.Description,
		OriginalPrice:     sheinParseFlexiblePrice(prod.OriginalPrice.String()),
		SpecialPrice:      sheinParseFlexiblePrice(prod.SpecialPrice.String()),
		PriceCurrency:     prod.PriceCurrency,
		Stock:             prod.Stock.String(),
		Brand:             prod.Brand,
		Category:          prod.Category,
		MainImageURL:      prod.MainImageURL,
		ImageURLs:         sheinRawJSONString(prod.ImageURLs),
		Attributes:        sheinRawJSONString(prod.Attributes),
		PlatformStatus:    prod.PlatformStatus,
		PlatformData:      sheinRawJSONString(prod.PlatformData),
		PlatformProductID: prod.PlatformProductID,
	}
	if prod.ShelfStatus != 0 {
		item.ShelfStatus = &prod.ShelfStatus
	}
	if prod.CategoryID != 0 {
		item.CategoryID = &prod.CategoryID
	}
	item.PublishTime = sheinFlexibleTimePtr(prod.PublishTime)
	item.ShelfTime = sheinFlexibleTimePtr(prod.ShelfTime)
	item.CreateTime = sheinFlexibleTimePtr(prod.CreateTime)
	item.UpdateTime = sheinFlexibleTimePtr(prod.UpdateTime)
	return item
}

func sheinPtrInt64(v int64) *int64 { return &v }

func sheinParseFlexiblePrice(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return value
}

func sheinRawJSONString(raw string) json.RawMessage {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	return json.RawMessage(trimmed)
}

func sheinFlexibleTimePtr(ft *types.FlexibleTime) *time.Time {
	if ft == nil || ft.IsZero() {
		return nil
	}
	value := ft.Time
	return &value
}
