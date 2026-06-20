package publish

import (
	"testing"

	"task-processor/internal/listingruntime"
)

func TestPublishMappingDTO(t *testing.T) {
	sku := "SKU-1"
	platformProductID := "PP-1"
	mapping := &listingruntime.ProductImportMapping{
		ID:                1,
		ImportTaskID:      2,
		StoreID:           3,
		Platform:          "SHEIN",
		Region:            "US",
		ProductID:         "ASIN-1",
		SKU:               &sku,
		Status:            0,
		TenantID:          4,
		PlatformProductID: &platformProductID,
	}

	dto := publishMappingDTO(mapping)
	if dto == nil || dto.ProductID != "ASIN-1" || dto.SKU == nil || *dto.SKU != "SKU-1" {
		t.Fatalf("publishMappingDTO() = %+v", dto)
	}
}
