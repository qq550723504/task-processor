package listingkit

import (
	"sort"
	"strings"
)

func buildSheinAttributeFreshnessOutcome(
	issueState sheinAttributeFreshnessIssueState,
	templateContext sheinAttributeFreshnessTemplateContext,
) (bool, string) {
	if len(issueState.invalid) > 0 || len(issueState.missing) > 0 {
		parts := []string{"当前普通属性模板已变化，现有 resolved attributes 中有内容已不再满足当前提交要求"}
		if len(issueState.invalid) > 0 {
			sort.Strings(issueState.invalid)
			parts = append(parts, "当前模板已失效的属性值: "+strings.Join(issueState.invalid, "; "))
			if drift := buildResolvedAttributeTemplateDriftDetails(issueState.invalidItems, templateContext.attributeIndex); drift != "" {
				parts = append(parts, "同属性在线模板差异: "+drift)
			}
		}
		if len(issueState.missing) > 0 {
			sort.Strings(issueState.missing)
			parts = append(parts, "当前模板新增或恢复生效的必填属性: "+strings.Join(issueState.missing, "; "))
		}
		return false, strings.Join(parts, "；")
	}
	return true, "当前普通属性模板中的已选值仍然合法"
}
