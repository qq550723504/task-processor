package sync

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/listingadmin"
	"task-processor/internal/pkg/types"
)

func (prod *TemuInventoryProductSnapshot) toBatchUpdateAttributesReq(attributes string) *listingadmin.ProductDataBatchUpdateAttributesReqDTO {
	return &listingadmin.ProductDataBatchUpdateAttributesReqDTO{
		Platform: prod.Platform,
		TenantID: prod.TenantID,
		StoreID:  prod.StoreID,
		Region:   prod.Region,
		Products: []listingadmin.ProductAttributesItemDTO{{
			PlatformProductID: prod.PlatformProductID,
			Attributes:        attributes,
		}},
	}
}

func temuInventoryProductDataFromSnapshot(prod *TemuInventoryProductSnapshot) listingadmin.ProductData {
	item := listingadmin.ProductData{
		TenantID:          prod.TenantID,
		StoreID:           temuSyncPtrInt64(prod.StoreID),
		Platform:          prod.Platform,
		Region:            prod.Region,
		ParentProductID:   prod.ParentProductID,
		ProductID:         prod.ProductID,
		Title:             prod.Title,
		Description:       prod.Description,
		OriginalPrice:     temuSyncParseFlexiblePrice(prod.OriginalPrice.String()),
		SpecialPrice:      temuSyncParseFlexiblePrice(prod.SpecialPrice.String()),
		PriceCurrency:     prod.PriceCurrency,
		Stock:             prod.Stock.String(),
		Brand:             prod.Brand,
		Category:          prod.Category,
		MainImageURL:      prod.MainImageURL,
		ImageURLs:         temuSyncRawJSONString(prod.ImageURLs),
		Attributes:        temuSyncRawJSONString(prod.Attributes),
		PlatformStatus:    prod.PlatformStatus,
		PlatformData:      temuSyncRawJSONString(prod.PlatformData),
		PlatformProductID: prod.PlatformProductID,
	}
	if prod.ShelfStatus != 0 {
		item.ShelfStatus = &prod.ShelfStatus
	}
	if prod.CategoryID != 0 {
		item.CategoryID = &prod.CategoryID
	}
	item.PublishTime = temuSyncFlexibleTimePtr(prod.PublishTime)
	item.ShelfTime = temuSyncFlexibleTimePtr(prod.ShelfTime)
	item.CreateTime = temuSyncFlexibleTimePtr(prod.CreateTime)
	item.UpdateTime = temuSyncFlexibleTimePtr(prod.UpdateTime)
	return item
}

func temuInventoryProductSnapshotFromProductData(prod *listingadmin.ProductData) *TemuInventoryProductSnapshot {
	if prod == nil {
		return nil
	}
	return &TemuInventoryProductSnapshot{
		ID:                prod.ID,
		Source:            prod.Source,
		ImportTaskID:      temuSyncInt64FromPtr(prod.ImportTaskID),
		StoreID:           temuSyncInt64FromPtr(prod.StoreID),
		Platform:          prod.Platform,
		CategoryID:        temuSyncInt64FromPtr(prod.CategoryID),
		Region:            prod.Region,
		ParentProductID:   prod.ParentProductID,
		ProductID:         prod.ProductID,
		Title:             prod.Title,
		Description:       prod.Description,
		OriginalPrice:     types.FlexibleString(temuSyncFloatToString(prod.OriginalPrice)),
		SpecialPrice:      types.FlexibleString(temuSyncFloatToString(prod.SpecialPrice)),
		PriceCurrency:     prod.PriceCurrency,
		Stock:             types.FlexibleString(prod.Stock),
		Brand:             prod.Brand,
		Category:          prod.Category,
		MainImageURL:      prod.MainImageURL,
		ImageURLs:         string(prod.ImageURLs),
		Attributes:        string(prod.Attributes),
		SourceURL:         prod.SourceURL,
		Status:            prod.Status,
		RawJSONDataID:     temuSyncInt64FromPtr(prod.RawJSONDataID),
		PlatformProductID: prod.PlatformProductID,
		PlatformStatus:    prod.PlatformStatus,
		ShelfStatus:       temuSyncIntFromPtr(prod.ShelfStatus),
		PublishTime:       types.ToFlexibleTime(prod.PublishTime),
		ShelfTime:         types.ToFlexibleTime(prod.ShelfTime),
		LastSyncTime:      types.ToFlexibleTime(prod.LastSyncTime),
		PlatformData:      string(prod.PlatformData),
		TenantID:          prod.TenantID,
		CreateTime:        types.ToFlexibleTime(prod.CreateTime),
		UpdateTime:        types.ToFlexibleTime(prod.UpdateTime),
	}
}

func temuSyncPtrInt64(v int64) *int64 { return &v }

func temuSyncParseFlexiblePrice(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return value
}

func temuSyncRawJSONString(raw string) json.RawMessage {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	return json.RawMessage(trimmed)
}

func temuSyncFlexibleTimePtr(ft *types.FlexibleTime) *time.Time {
	if ft == nil || ft.IsZero() {
		return nil
	}
	value := ft.Time
	return &value
}

func temuSyncInt64FromPtr(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func temuSyncIntFromPtr(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func temuSyncFloatToString(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
