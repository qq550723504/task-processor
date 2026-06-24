package listingkit

import (
	"sort"
	"strings"

	"task-processor/internal/catalog/canonical"
)

func sdsRetirementTaskMatchesIdentity(task *Task, identity SDSBaselineIdentity) bool {
	if task == nil || task.Request == nil || task.Request.Options == nil || task.Request.Options.SDS == nil {
		return false
	}
	taskIdentity := sdsBaselineIdentityFromOptions(task.Request.Options.SDS)
	expected := identity
	expected.SelectedVariantIDs = normalizedSDSBaselineVariantIDs(expected.SelectedVariantIDs)
	return taskIdentity.ParentProductID == expected.ParentProductID &&
		taskIdentity.PrototypeGroupID == expected.PrototypeGroupID &&
		taskIdentity.VariantID == expected.VariantID &&
		sdsRetirementInt64SlicesEqual(taskIdentity.SelectedVariantIDs, expected.SelectedVariantIDs)
}

func sdsRetirementInt64SlicesEqual(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func sdsRetirementSourceSKUSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized != "" {
			out[normalized] = struct{}{}
		}
	}
	return out
}

func sdsRetirementSortedSetValues(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func sdsRetirementSourceSKUs(product *canonical.Product) []string {
	if product == nil {
		return nil
	}
	set := map[string]struct{}{}
	for _, variant := range product.Variants {
		if value, ok := variant.Attributes["source_sds_sku"]; ok {
			normalized := strings.TrimSpace(value.Value)
			if normalized != "" {
				set[normalized] = struct{}{}
			}
		}
	}
	return sdsRetirementSortedSetValues(set)
}
