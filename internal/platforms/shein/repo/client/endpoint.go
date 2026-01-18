package client

// API 路径常量
const (
	apiPrefix  = "/spmp-api-prefix/spmp"
	dpasPrefix = "/dpas-api-prefix/dpas"
	ssoPrefix  = "/sso-prefix"
	mbrsPrefix = "/mrs-api-prefix/mbrs"
	//certificateRuleEndpoint        = apiPrefix + "/certificate/get_certificate_rule"
	productRecordEndpoint          = apiPrefix + "/product/publish/record/page_list"
	batchCheckOnWayEndpoint        = apiPrefix + "/document/web/batch_check_on_way"
	validateAttributeEndpoint      = apiPrefix + "/attribute/valid_pre_custom_attribute_value"
	addAttributeValueEndpoint      = apiPrefix + "/attribute/add_pre_custom_attribute_value"
	getCategoryEndpoint            = apiPrefix + "/product/get_all_category"
	getCategoryTreeEndpoint        = apiPrefix + "/supplier/query_category_tree"
	getProductEndpoint             = apiPrefix + "/product/get_product"
	updateProductEndpoint          = apiPrefix + "/product/update_product"
	deleteProductEndpoint          = apiPrefix + "/product/delete_product"
	confirmPublishEndpoint         = apiPrefix + "/product/confirm_publish"
	saveDraftEndpoint              = apiPrefix + "/product/save_draft"
	publishProductEndpoint         = apiPrefix + "/product/publish"
	uploadImageEndpoint            = apiPrefix + "/image/upload_image"
	translateTextEndpoint          = apiPrefix + "/abc/google_text_translate"
	getWarehousesEndpoint          = apiPrefix + "/inventory/query_merchant_warehouse"
	getAttributeTemplatesEndpoint  = apiPrefix + "/basic/query_attribute_templates"
	getPartInfoEndpoint            = apiPrefix + "/part/get_part_info"
	batchHandleCostDiscussEndpoint = dpasPrefix + "/discuss/batch_handle_cost_discuss"
	bargainPageNewEndpoint         = dpasPrefix + "/discuss/bargain_page"
	getSpuLimitCountEndpoint       = apiPrefix + "/basic/get_spu_limit_count"
	getUserEndpoint                = ssoPrefix + "/sso-prefix/auth/getUser"
	getSupplierOperateInfoEndpoint = "/sso/public/account/supplier/getSupplierOperateInfo"
	productListEndpoint            = apiPrefix + "/product/list"
	queryStockEndpoint             = apiPrefix + "/product/query_msc_stock_for_released_page"
	queryPriceEndpoint             = apiPrefix + "/product/query_price"
	queryCostPriceEndpoint         = apiPrefix + "/product/price/query_cost_price"
	queryInventoryEndpoint         = apiPrefix + "/product/inventory/query"
	updateInventoryEndpoint        = apiPrefix + "/product/inventory/update"
	operateShelfStatusEndpoint     = apiPrefix + "/product/operate_Shelf_status"
	// 自动报名活动
	getAvailableSkcListEndpoint = mbrsPrefix + "/activity/auto_partake/get_available_skc_list"
	saveConfigEndpoint          = mbrsPrefix + "/activity/auto_partake/save_config"
	getConfigListEndpoint       = mbrsPrefix + "/activity/auto_partake/get_config_list"

	// 促销活动相关
	mrsPrefix                   = "/mrs-api-prefix/promotion"
	queryPromotionGoodsEndpoint = mrsPrefix + "/simple_platform/query_goods"
	createActivityEndpoint      = mrsPrefix + "/simple_platform/create_activity"

	// 资金损益相关
	capitalPrefix                = "/mrs-api-prefix/capital"
	calculateSupplyPriceEndpoint = capitalPrefix + "/loss/calculate_supply_price"

	// 商品上架配额相关
	queryShelfQuotaEndpoint = apiPrefix + "/product/skc/shelf/query_quota"
)
