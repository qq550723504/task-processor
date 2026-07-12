package client

// GetEndpoint 获取API端点
func GetEndpoint(name string) string {
	endpoints := map[string]string{
		"productRecord":           productRecordEndpoint,
		"batchCheckOnWay":         batchCheckOnWayEndpoint,
		"validateAttribute":       validateAttributeEndpoint,
		"addAttributeValue":       addAttributeValueEndpoint,
		"getCategory":             getCategoryEndpoint,
		"getCategoryTree":         getCategoryTreeEndpoint,
		"cvTextSuggestCategory":   cvTextSuggestCategoryEndpoint,
		"getProduct":              getProductEndpoint,
		"queryBrandList":          queryBrandListEndpoint,
		"updateProduct":           updateProductEndpoint,
		"deleteProduct":           deleteProductEndpoint,
		"confirmPublish":          confirmPublishEndpoint,
		"saveDraft":               saveDraftEndpoint,
		"publishProduct":          publishProductEndpoint,
		"productNameLengthConfig": productNameLengthConfigEndpoint,
		"uploadImage":             uploadImageEndpoint,
		"translateText":           translateTextEndpoint,
		"getWarehouses":           getWarehousesEndpoint,
		"storeAddressList":        storeAddressListEndpoint,
		"storeAddressAdd":         storeAddressAddEndpoint,
		"storeAddressCheck":       storeAddressCheckEndpoint,
		"getAttributeTemplates":   getAttributeTemplatesEndpoint,
		"getPartInfo":             getPartInfoEndpoint,
		"batchHandleCostDiscuss":  batchHandleCostDiscussEndpoint,
		"bargainPageNew":          bargainPageNewEndpoint,
		"getSpuLimitCount":        getSpuLimitCountEndpoint,
		"getUser":                 getUserEndpoint,
		"getSupplierOperateInfo":  getSupplierOperateInfoEndpoint,
		"productList":             productListEndpoint,
		"queryStock":              queryStockEndpoint,
		"queryPrice":              queryPriceEndpoint,
		"queryCostPrice":          queryCostPriceEndpoint,
		"queryInventory":          queryInventoryEndpoint,
		"updateInventory":         updateInventoryEndpoint,
		"operateShelfStatus":      operateShelfStatusEndpoint,
		"getAvailableSkcList":     getAvailableSkcListEndpoint,
		"saveConfig":              saveConfigEndpoint,
		"getConfigList":           getConfigListEndpoint,
		"updateConfigState":       updateConfigStateEndpoint,
		"queryPromotionGoods":     queryPromotionGoodsEndpoint,
		"calculateSupplyPrice":    calculateSupplyPriceEndpoint,
		"createActivity":          createActivityEndpoint,
		"queryShelfQuota":         queryShelfQuotaEndpoint,
	}

	return endpoints[name]
}

// 为了向后兼容，提供具体的getter方法
func GetProductRecordEndpoint() string           { return productRecordEndpoint }
func GetBatchCheckOnWayEndpoint() string         { return batchCheckOnWayEndpoint }
func GetValidateAttributeEndpoint() string       { return validateAttributeEndpoint }
func GetAddAttributeValueEndpoint() string       { return addAttributeValueEndpoint }
func GetCategoryEndpoint() string                { return getCategoryEndpoint }
func GetCategoryTreeEndpoint() string            { return getCategoryTreeEndpoint }
func GetCvTextSuggestCategoryEndpoint() string   { return cvTextSuggestCategoryEndpoint }
func GetProductEndpoint() string                 { return getProductEndpoint }
func GetQueryBrandListEndpoint() string          { return queryBrandListEndpoint }
func GetUpdateProductEndpoint() string           { return updateProductEndpoint }
func GetDeleteProductEndpoint() string           { return deleteProductEndpoint }
func GetConfirmPublishEndpoint() string          { return confirmPublishEndpoint }
func GetSaveDraftEndpoint() string               { return saveDraftEndpoint }
func GetPublishProductEndpoint() string          { return publishProductEndpoint }
func GetProductNameLengthConfigEndpoint() string { return productNameLengthConfigEndpoint }
func GetUploadImageEndpoint() string             { return uploadImageEndpoint }
func GetTranslateTextEndpoint() string           { return translateTextEndpoint }
func GetWarehousesEndpoint() string              { return getWarehousesEndpoint }
func GetStoreAddressListEndpoint() string        { return storeAddressListEndpoint }
func GetStoreAddressAddEndpoint() string         { return storeAddressAddEndpoint }
func GetStoreAddressCheckEndpoint() string       { return storeAddressCheckEndpoint }
func GetAttributeTemplatesEndpoint() string      { return getAttributeTemplatesEndpoint }
func GetPartInfoEndpoint() string                { return getPartInfoEndpoint }
func GetBatchHandleCostDiscussEndpoint() string  { return batchHandleCostDiscussEndpoint }
func GetBargainPageNewEndpoint() string          { return bargainPageNewEndpoint }
func GetBatchReQuoteEndpoint() string            { return batchReQuoteEndpoint }
func GetSpuLimitCountEndpoint() string           { return getSpuLimitCountEndpoint }
func GetUserEndpoint() string                    { return getUserEndpoint }
func GetSupplierOperateInfoEndpoint() string     { return getSupplierOperateInfoEndpoint }
func GetProductListEndpoint() string             { return productListEndpoint }
func GetQueryStockEndpoint() string              { return queryStockEndpoint }
func GetQueryPriceEndpoint() string              { return queryPriceEndpoint }
func GetQueryCostPriceEndpoint() string          { return queryCostPriceEndpoint }
func GetQueryInventoryEndpoint() string          { return queryInventoryEndpoint }
func GetUpdateInventoryEndpoint() string         { return updateInventoryEndpoint }
func GetOperateShelfStatusEndpoint() string      { return operateShelfStatusEndpoint }
func GetAvailableSkcListEndpoint() string        { return getAvailableSkcListEndpoint }
func GetSaveConfigEndpoint() string              { return saveConfigEndpoint }
func GetConfigListEndpoint() string              { return getConfigListEndpoint }
func GetUpdateConfigStateEndpoint() string       { return updateConfigStateEndpoint }
func GetQueryPromotionGoodsEndpoint() string     { return queryPromotionGoodsEndpoint }
func GetCalculateSupplyPriceEndpoint() string    { return calculateSupplyPriceEndpoint }
func GetCreateActivityEndpoint() string          { return createActivityEndpoint }
func GetQueryShelfQuotaEndpoint() string         { return queryShelfQuotaEndpoint }
