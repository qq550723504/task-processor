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
	options := task.Request.Options.SDS
	return options.ParentProductID == identity.ParentProductID &&
		options.PrototypeGroupID == identity.PrototypeGroupID &&
		options.VariantID == identity.VariantID
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
