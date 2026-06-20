package inventory

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/listingadmin"
	"task-processor/internal/pkg/types"
)

func inventoryProductDataFromSnapshot(prod *InventoryProductSnapshot) listingadmin.ProductData {
	item := listingadmin.ProductData{
		TenantID:          prod.TenantID,
		StoreID:           sheinInvPtrInt64(prod.StoreID),
		Platform:          prod.Platform,
		Region:            prod.Region,
		ParentProductID:   prod.ParentProductID,
		ProductID:         prod.ProductID,
		Title:             prod.Title,
		Description:       prod.Description,
		OriginalPrice:     sheinInvParseFlexiblePrice(prod.OriginalPrice.String()),
		SpecialPrice:      sheinInvParseFlexiblePrice(prod.SpecialPrice.String()),
		PriceCurrency:     prod.PriceCurrency,
		Stock:             prod.Stock.String(),
		Brand:             prod.Brand,
		Category:          prod.Category,
		MainImageURL:      prod.MainImageURL,
		ImageURLs:         sheinInvRawJSONString(prod.ImageURLs),
		Attributes:        sheinInvRawJSONString(prod.Attributes),
		PlatformStatus:    prod.PlatformStatus,
		PlatformData:      sheinInvRawJSONString(prod.PlatformData),
		PlatformProductID: prod.PlatformProductID,
	}
	if prod.ShelfStatus != 0 {
		item.ShelfStatus = &prod.ShelfStatus
	}
	if prod.CategoryID != 0 {
		item.CategoryID = &prod.CategoryID
	}
	item.PublishTime = sheinInvFlexibleTimePtr(prod.PublishTime)
	item.ShelfTime = sheinInvFlexibleTimePtr(prod.ShelfTime)
	item.CreateTime = sheinInvFlexibleTimePtr(prod.CreateTime)
	item.UpdateTime = sheinInvFlexibleTimePtr(prod.UpdateTime)
	return item
}

func inventoryProductSnapshotFromProductData(prod *listingadmin.ProductData) *InventoryProductSnapshot {
	if prod == nil {
		return nil
	}
	return &InventoryProductSnapshot{
		ID:                prod.ID,
		Source:            prod.Source,
		ImportTaskID:      sheinInvInt64FromPtr(prod.ImportTaskID),
		StoreID:           sheinInvInt64FromPtr(prod.StoreID),
		Platform:          prod.Platform,
		CategoryID:        sheinInvInt64FromPtr(prod.CategoryID),
		Region:            prod.Region,
		ParentProductID:   prod.ParentProductID,
		ProductID:         prod.ProductID,
		Title:             prod.Title,
		Description:       prod.Description,
		OriginalPrice:     types.FlexibleString(sheinInvFloatToString(prod.OriginalPrice)),
		SpecialPrice:      types.FlexibleString(sheinInvFloatToString(prod.SpecialPrice)),
		PriceCurrency:     prod.PriceCurrency,
		Stock:             types.FlexibleString(prod.Stock),
		Brand:             prod.Brand,
		Category:          prod.Category,
		MainImageURL:      prod.MainImageURL,
		ImageURLs:         string(prod.ImageURLs),
		Attributes:        string(prod.Attributes),
		SourceURL:         prod.SourceURL,
		Status:            prod.Status,
		RawJSONDataID:     sheinInvInt64FromPtr(prod.RawJSONDataID),
		PlatformProductID: prod.PlatformProductID,
		PlatformStatus:    prod.PlatformStatus,
		ShelfStatus:       sheinInvIntFromPtr(prod.ShelfStatus),
		PublishTime:       types.ToFlexibleTime(prod.PublishTime),
		ShelfTime:         types.ToFlexibleTime(prod.ShelfTime),
		LastSyncTime:      types.ToFlexibleTime(prod.LastSyncTime),
		PlatformData:      string(prod.PlatformData),
		TenantID:          prod.TenantID,
		CreateTime:        types.ToFlexibleTime(prod.CreateTime),
		UpdateTime:        types.ToFlexibleTime(prod.UpdateTime),
	}
}

func sheinInvPtrInt64(v int64) *int64 { return &v }

func sheinInvParseFlexiblePrice(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return value
}

func sheinInvRawJSONString(raw string) json.RawMessage {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	return json.RawMessage(trimmed)
}

func sheinInvFlexibleTimePtr(ft *types.FlexibleTime) *time.Time {
	if ft == nil || ft.IsZero() {
		return nil
	}
	value := ft.Time
	return &value
}

func sheinInvInt64FromPtr(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func sheinInvIntFromPtr(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func sheinInvFloatToString(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
