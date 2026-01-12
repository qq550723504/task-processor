package models

// ProductSubmitRequest TEMU产品提交请求结构体（完整版）
type ProductSubmitRequest struct {
	GoodsBasic            GoodsBasicInfo `json:"goods_basic"`
	GoodsSaleInfo         GoodsSaleInfo  `json:"goods_sale_info"`
	GoodsServicePromise   ServicePromise `json:"goods_service_promise"`
	GoodsExtensionInfo    ExtensionInfo  `json:"goods_extension_info"`
	Extra                 Extra          `json:"extra"`
	CanSave               bool           `json:"can_save"`
	SupportMaxRetailPrice bool           `json:"support_max_retail_price"`
	PlatformExpressBill   bool           `json:"platform_express_bill"`
	SkcList               []Skc          `json:"skc_list"`
	//BatchSkuInfo          BatchSkuInfo   `json:"batch_sku_info"`
}

// ProductSubmitResponse TEMU产品提交响应结构体
type ProductSubmitResponse struct {
	Success   bool                 `json:"success"`
	ErrorCode int                  `json:"error_code"`
	Message   string               `json:"error_msg,omitempty"`
	Result    *ProductSubmitResult `json:"result,omitempty"`
}

// ProductSubmitResult 产品提交结果
type ProductSubmitResult struct {
	ListingCommitID      string `json:"listing_commit_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	GoodsCommitID        string `json:"goods_commit_id"`
	Status               int    `json:"status"`
	Message              string `json:"message,omitempty"`
}

// CreateCommitRequest 创建提交请求
type CreateCommitRequest struct {
	CatIDs      []int  `json:"cat_ids"`
	CatID       int    `json:"cat_id"`
	GoodsName   string `json:"goods_name"`
	OperateType int    `json:"operate_type"`
}

// CreateCommitResponse 创建提交响应
type CreateCommitResponse struct {
	Success   bool                `json:"success"`
	ErrorCode int                 `json:"error_code"`
	Message   string              `json:"error_msg,omitempty"`
	Result    *CreateCommitResult `json:"result,omitempty"`
}

// CommitCreateResponse 创建提交响应（别名，保持兼容性）
type CommitCreateResponse = CreateCommitResponse

// CommitCreateResult 创建提交结果（别名，保持兼容性）
type CommitCreateResult = CreateCommitResult

// CreateCommitResult 创建提交结果
type CreateCommitResult struct {
	GoodsID              string `json:"goods_id"`
	ListingCommitID      string `json:"listing_commit_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	GoodsCommitID        string `json:"goods_commit_id"`
}
