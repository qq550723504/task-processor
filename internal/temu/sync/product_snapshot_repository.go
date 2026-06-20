// package sync 提供 TEMU 产品快照到 listingadmin 仓储模型的转换
package sync

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/listingadmin"
	"task-processor/internal/pkg/types"
)

func temuProductDataFromSnapshot(prod *TemuProductSnapshot) listingadmin.ProductData {
	item := listingadmin.ProductData{
		TenantID:          prod.TenantID,
		StoreID:           ptrInt64(prod.StoreID),
		Platform:          prod.Platform,
		Region:            prod.Region,
		ParentProductID:   prod.ParentProductID,
		ProductID:         prod.ProductID,
		Title:             prod.Title,
		Description:       prod.Description,
		OriginalPrice:     parseFlexiblePrice(prod.OriginalPrice.String()),
		SpecialPrice:      parseFlexiblePrice(prod.SpecialPrice.String()),
		PriceCurrency:     prod.PriceCurrency,
		Stock:             prod.Stock.String(),
		Brand:             prod.Brand,
		Category:          prod.Category,
		MainImageURL:      prod.MainImageURL,
		ImageURLs:         rawJSONString(prod.ImageURLs),
		Attributes:        rawJSONString(prod.Attributes),
		PlatformStatus:    prod.PlatformStatus,
		PlatformData:      rawJSONString(prod.PlatformData),
		PlatformProductID: prod.PlatformProductID,
	}
	if prod.ShelfStatus != 0 {
		item.ShelfStatus = &prod.ShelfStatus
	}
	if prod.CategoryID != 0 {
		item.CategoryID = &prod.CategoryID
	}
	item.PublishTime = flexibleTimePtr(prod.PublishTime)
	item.ShelfTime = flexibleTimePtr(prod.ShelfTime)
	item.CreateTime = flexibleTimePtr(prod.CreateTime)
	item.UpdateTime = flexibleTimePtr(prod.UpdateTime)
	return item
}

func ptrInt64(v int64) *int64 { return &v }

func parseFlexiblePrice(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return value
}

func rawJSONString(raw string) json.RawMessage {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	return json.RawMessage(trimmed)
}

func flexibleTimePtr(ft *types.FlexibleTime) *time.Time {
	if ft == nil || ft.IsZero() {
		return nil
	}
	value := ft.Time
	return &value
}
