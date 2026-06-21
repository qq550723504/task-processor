package workspace

import "strings"

const (
	FreshnessAuthKey          = "shein_online_auth"
	FreshnessCategoryKey      = "shein_category_template_freshness"
	FreshnessAttributeKey     = "shein_attribute_template_freshness"
	FreshnessSaleAttributeKey = "shein_sale_attribute_freshness"
)

// BuildFreshnessAuthFailureCheck builds a blocking readiness check for SHEIN auth failures.
func BuildFreshnessAuthFailureCheck(err error) ReadinessCheckSpec {
	message := "SHEIN 提交店铺当前不可用，请先刷新登录态后再提交"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message += "：" + strings.TrimSpace(err.Error())
	}
	return ReadinessCheckSpec{
		Key:             FreshnessAuthKey,
		Label:           "SHEIN 在线登录态",
		OK:              false,
		Message:         message,
		FieldPaths:      []string{"shein.store_resolution", "shein.review_notes"},
		SuggestedAction: "重新登录 SHEIN 店铺",
	}
}

// BuildFreshnessAuthSuccessCheck builds a passing readiness check for SHEIN auth.
func BuildFreshnessAuthSuccessCheck() ReadinessCheckSpec {
	return ReadinessCheckSpec{
		Key:             FreshnessAuthKey,
		Label:           "SHEIN 在线登录态",
		OK:              true,
		Message:         "SHEIN 提交店铺当前可用",
		FieldPaths:      []string{"shein.store_resolution"},
		SuggestedAction: "重新登录 SHEIN 店铺",
	}
}

// BuildFreshnessCategoryCheck builds a readiness check for category template freshness.
func BuildFreshnessCategoryCheck(ok bool, message string) ReadinessCheckSpec {
	return ReadinessCheckSpec{
		Key:             FreshnessCategoryKey,
		Label:           "类目模板新鲜度",
		OK:              ok,
		Message:         message,
		FieldPaths:      []string{"shein.category_id", "shein.category_id_list", "shein.product_type_id"},
		SuggestedAction: "刷新类目模板",
	}
}

// BuildFreshnessAttributeCheck builds a readiness check for display attribute template freshness.
func BuildFreshnessAttributeCheck(ok bool, message string) ReadinessCheckSpec {
	return ReadinessCheckSpec{
		Key:             FreshnessAttributeKey,
		Label:           "普通属性模板新鲜度",
		OK:              ok,
		Message:         message,
		FieldPaths:      []string{"shein.resolved_attributes", "shein.attribute_resolution"},
		SuggestedAction: "刷新属性模板",
	}
}

// BuildFreshnessSaleAttributeCheck builds a readiness check for sale attribute freshness.
func BuildFreshnessSaleAttributeCheck(ok bool, message string) ReadinessCheckSpec {
	return ReadinessCheckSpec{
		Key:             FreshnessSaleAttributeKey,
		Label:           "销售属性模板新鲜度",
		OK:              ok,
		Message:         message,
		FieldPaths:      []string{"shein.sale_attribute_resolution", "shein.request_draft.skc_list"},
		SuggestedAction: "刷新销售属性",
	}
}
