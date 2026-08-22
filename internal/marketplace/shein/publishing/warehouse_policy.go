package publishing

import (
	"strings"

	common "task-processor/internal/publishing/common"
)

const defaultSubmitWarehouseCode = "DEFAULT"

// WarehouseOption keeps the marketplace-facing name while sharing the
// platform-neutral contract with publishing/common.
type WarehouseOption = common.WarehouseOption

// SelectWarehouseCodeForSite returns the first warehouse that sells to site.
// When no warehouse matches, it preserves the legacy first-warehouse fallback.
func SelectWarehouseCodeForSite(warehouses []WarehouseOption, site string) string {
	return common.SelectWarehouseCodeForSite(warehouses, site)
}

// SubmitPreferredWarehouseCode returns the first configured warehouse code or the SHEIN default sentinel.
func SubmitPreferredWarehouseCode(warehouseCode string) string {
	for _, item := range strings.Split(warehouseCode, ",") {
		value := strings.TrimSpace(item)
		if value != "" {
			return value
		}
	}
	return defaultSubmitWarehouseCode
}
