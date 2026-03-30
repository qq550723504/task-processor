package pipeline

import (
	"strings"

	shein "task-processor/internal/shein"
)

func resolveStageName(handlerName string) string {
	switch strings.TrimSpace(handlerName) {
	case "获取店铺信息":
		return "init_store"
	case "获取并缓存主产品数据":
		return "fetch_product"
	case "检查产品是否已上架":
		return "check_existing_product"
	case "验证图片":
		return "validate_images"
	case "初始化产品数据":
		return "init_product_data"
	case "获取供应商信息":
		return "load_supplier_info"
	case "处理店铺ID":
		return "resolve_store_id"
	case "检查SKC上架额度":
		return "check_shelf_quota"
	case "验证任务":
		return "validate_rules"
	case "应用筛选规则":
		return "apply_filter_rule"
	case "查询是否有发品记录":
		return "check_spu_record"
	case "获取并缓存变体数据":
		return "fetch_variants"
	case "重新应用筛选规则到变体":
		return "reapply_filter_rule"
	case "检查每日上架限制":
		return "check_daily_limit"
	case "获取分类树":
		return "load_category_tree"
	case "AI选择分类":
		return "select_category"
	case "获取仓库信息":
		return "load_warehouse"
	case "翻译标题描述":
		return "translate_content"
	case "设置站点信息":
		return "load_site_info"
	case "获取属性模板":
		return "load_attribute_templates"
	case "构建属性信息":
		return "build_attributes"
	case "AI选择属性":
		return "select_attributes"
	case "填充属性":
		return "fill_attributes"
	case "AI生成销售规格":
		return "build_sale_attributes"
	case "验证修复销售属性":
		return "repair_sale_attributes"
	case "构建SKC列表":
		return "build_skc_list"
	case "构建最终的发品数据":
		return "build_publish_payload"
	case "清理敏感词":
		return "clean_sensitive_words"
	case "发布产品":
		return "publish_product"
	case "标记变体构建成功":
		return "mark_variant_publish_success"
	case "收集分类限制":
		return "collect_category_restrictions"
	case "保存发品成功后返回的信息":
		return "save_publish_result"
	default:
		return "unknown"
	}
}

func formatStageStatusMessage(stage, reasonMessage string) string {
	return shein.FormatTaskStageMessage(stage, reasonMessage)
}
