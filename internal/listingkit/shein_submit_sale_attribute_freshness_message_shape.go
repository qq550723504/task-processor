package listingkit

import (
	"sort"
	"strings"
)

func buildSheinSaleAttributeFreshnessResolutionOutcome(
	baseIssues []string,
	invalidState sheinSaleAttributeFreshnessInvalidState,
) (bool, string, bool) {
	issues := append([]string(nil), baseIssues...)
	if len(invalidState.invalidSKC) > 0 {
		sort.Strings(invalidState.invalidSKC)
		issues = append(issues, "当前模板已失效的 SKC 销售属性值: "+strings.Join(invalidState.invalidSKC, "; "))
	}
	if len(invalidState.invalidSKU) > 0 {
		sort.Strings(invalidState.invalidSKU)
		issues = append(issues, "当前模板已失效的 SKU 销售属性值: "+strings.Join(invalidState.invalidSKU, "; "))
	}

	if len(issues) > 0 {
		return false, "当前销售属性模板已变化，现有销售属性中有内容已不再满足当前提交要求；" + strings.Join(issues, "；"), invalidState.changed
	}
	if invalidState.changed {
		return true, "当前销售属性模板中的已选值仍然合法，失效值已通过 SHEIN 自定义销售属性校验自动修正", true
	}
	return true, "当前销售属性模板中的已选值仍然合法", false
}
