package submission

import "strings"

// SourceFactsReady evaluates whether source-derived facts are ready for submit.
func SourceFactsReady(metadata map[string]string) (bool, string) {
	if metadata == nil {
		return true, "来源事实无需额外复核"
	}
	if strings.ToLower(strings.TrimSpace(metadata["source_platform"])) != "1688" {
		return true, "来源事实无需额外复核"
	}
	if strings.ToLower(strings.TrimSpace(metadata["source_fact_review_required"])) != "true" {
		return true, "1688 来源事实已具备抓取依据"
	}
	fields := strings.TrimSpace(metadata["source_fact_review_fields"])
	if fields == "" {
		return false, "1688 来源商品包含缺少抓取依据的 LLM 推断字段，提交前必须复核"
	}
	return false, "1688 来源商品存在缺少抓取依据的 LLM 推断字段，提交前必须复核：" + fields
}
