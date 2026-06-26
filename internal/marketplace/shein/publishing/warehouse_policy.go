package publishing

import "strings"

const defaultSubmitWarehouseCode = "DEFAULT"

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
