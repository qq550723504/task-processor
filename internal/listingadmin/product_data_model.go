package listingadmin

import (
	"encoding/json"
	"strings"
)

func (r listingProductData) toProductData() ProductData {
	return ProductData{
		ID:                r.ID,
		TenantID:          r.TenantID,
		Source:            r.Source,
		ImportTaskID:      int64PtrIfPositive(r.ImportTaskID),
		RawJSONDataID:     int64PtrIfPositive(r.RawJSONDataID),
		StoreID:           int64PtrIfPositive(r.StoreID),
		CategoryID:        int64PtrIfPositive(r.CategoryID),
		Platform:          r.Platform,
		Region:            r.Region,
		ParentProductID:   r.ParentProductID,
		ProductID:         r.ProductID,
		Title:             r.Title,
		Description:       r.Description,
		OriginalPrice:     r.OriginalPrice,
		SpecialPrice:      r.SpecialPrice,
		PriceCurrency:     r.PriceCurrency,
		Stock:             r.Stock,
		Brand:             r.Brand,
		Category:          r.Category,
		MainImageURL:      r.MainImageURL,
		ImageURLs:         rawJSONOrString(r.ImageURLs),
		Attributes:        rawJSONOrString(r.Attributes),
		SourceURL:         r.SourceURL,
		Status:            r.Status,
		PlatformProductID: r.PlatformProductID,
		PlatformStatus:    r.PlatformStatus,
		ShelfStatus:       intPtrIfPositive(r.ShelfStatus),
		PublishTime:       r.PublishTime,
		ShelfTime:         r.ShelfTime,
		LastSyncTime:      r.LastSyncTime,
		PlatformData:      rawJSONOrString(r.PlatformData),
		CreateTime:        r.CreateTime,
		UpdateTime:        r.UpdateTime,
	}
}

func listingProductDataFromProductData(product *ProductData) listingProductData {
	if product == nil {
		return listingProductData{}
	}
	return listingProductData{
		ID:                product.ID,
		TenantID:          product.TenantID,
		Source:            strings.TrimSpace(product.Source),
		ImportTaskID:      int64Value(product.ImportTaskID),
		RawJSONDataID:     int64Value(product.RawJSONDataID),
		StoreID:           int64Value(product.StoreID),
		CategoryID:        int64Value(product.CategoryID),
		Platform:          strings.TrimSpace(product.Platform),
		Region:            strings.TrimSpace(product.Region),
		ParentProductID:   strings.TrimSpace(product.ParentProductID),
		ProductID:         strings.TrimSpace(product.ProductID),
		Title:             strings.TrimSpace(product.Title),
		Description:       strings.TrimSpace(product.Description),
		OriginalPrice:     product.OriginalPrice,
		SpecialPrice:      product.SpecialPrice,
		PriceCurrency:     strings.TrimSpace(product.PriceCurrency),
		Stock:             strings.TrimSpace(product.Stock),
		Brand:             strings.TrimSpace(product.Brand),
		Category:          strings.TrimSpace(product.Category),
		MainImageURL:      strings.TrimSpace(product.MainImageURL),
		ImageURLs:         string(product.ImageURLs),
		Attributes:        string(product.Attributes),
		SourceURL:         strings.TrimSpace(product.SourceURL),
		Status:            product.Status,
		PlatformProductID: strings.TrimSpace(product.PlatformProductID),
		PlatformStatus:    strings.TrimSpace(product.PlatformStatus),
		ShelfStatus:       intValue(product.ShelfStatus),
		PublishTime:       product.PublishTime,
		ShelfTime:         product.ShelfTime,
		LastSyncTime:      product.LastSyncTime,
		PlatformData:      string(product.PlatformData),
	}
}

func applyProductDataAuditFields(row *listingProductData, userID string, includeCreate bool) {
	if row == nil || strings.TrimSpace(userID) == "" {
		return
	}
	userID = strings.TrimSpace(userID)
	row.OwnerUserID = userID
	row.Updater = userID
	row.UpdatedBy = userID
	if includeCreate {
		row.Creator = userID
		row.CreatedBy = userID
	}
}

func rawJSONOrString(value string) json.RawMessage {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if json.Valid([]byte(trimmed)) {
		return json.RawMessage(trimmed)
	}
	encoded, _ := json.Marshal(trimmed)
	return encoded
}
