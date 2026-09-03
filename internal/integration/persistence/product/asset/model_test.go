package assetpersistence

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	productasset "task-processor/internal/product/asset"
)

func TestIndexedIdentityColumnSizesMatchDomainLimit(t *testing.T) {
	for _, model := range []struct {
		value  any
		fields []string
	}{
		{value: ApprovedAssetRecord{}, fields: []string{"TenantID", "RunID", "SlotID", "ActionID", "AssetID", "ProductKey"}},
		{value: ApprovalReceiptRecord{}, fields: []string{"TenantID", "ActionID"}},
		{value: ApprovedInventoryHeadRecord{}, fields: []string{"TenantID", "ProductKey", "ActionID"}},
		{value: ApprovedInventoryVersionHeadRecord{}, fields: []string{"TenantID", "ProductKey", "ActionID"}},
	} {
		typeOfModel := reflect.TypeOf(model.value)
		for _, fieldName := range model.fields {
			field, ok := typeOfModel.FieldByName(fieldName)
			if !ok {
				t.Fatalf("%s missing field %s", typeOfModel.Name(), fieldName)
			}
			got, ok := gormTagInteger(field.Tag.Get("gorm"), "size")
			if !ok || got != productasset.MaxIdentityLength {
				t.Fatalf("%s.%s gorm size = %d (present=%v), want %d", typeOfModel.Name(), fieldName, got, ok, productasset.MaxIdentityLength)
			}
		}
	}
}

func gormTagInteger(tag, key string) (int, bool) {
	for _, option := range strings.Split(tag, ";") {
		name, value, found := strings.Cut(option, ":")
		if !found || name != key {
			continue
		}
		parsed, err := strconv.Atoi(value)
		return parsed, err == nil
	}
	return 0, false
}
