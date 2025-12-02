package api

// RawJsonDataReqDTO 原始JSON数据请求DTO
type RawJsonDataReqDTO struct {
	TenantID   int64  `json:"tenantId" binding:"required"`   // 租户编号
	Platform   string `json:"platform" binding:"required"`   // 平台
	ProductID  string `json:"productId" binding:"required"`  // 产品ID或ASIN
	Region     string `json:"region" binding:"required"`     // 区域
	StoreID    int64  `json:"storeId" binding:"required"`    // 店铺ID
	CategoryID int64  `json:"categoryId" binding:"required"` // 分类ID
	Creator    string `json:"creator" binding:"required"`    // 创建者
}

// RawJsonDataRespDTO 原始JSON数据响应DTO
type RawJsonDataRespDTO struct {
	ID          int64  `json:"id"`          // 主键ID
	Platform    string `json:"platform"`    // 平台
	ProductID   string `json:"productId"`   // 产品ID或ASIN
	Region      string `json:"region"`      // 区域
	RawJSONData string `json:"rawJsonData"` // 原生JSON数据
	CreateTime  int64  `json:"createTime"`  // 创建时间
}

// ProductVariantConfirmationReqDTO 产品变体确认请求DTO
type ProductVariantConfirmationReqDTO struct {
	ProductID  string   `json:"productId" binding:"required"`  // 产品ID或ASIN
	Platform   string   `json:"platform" binding:"required"`   // 平台类型
	Region     string   `json:"region" binding:"required"`     // 区域
	VariantIds []string `json:"variantIds" binding:"required"` // 变体ID列表
}

// RawJsonDataCreateReqDTO 原始JSON数据创建请求DTO
type RawJsonDataCreateReqDTO struct {
	TenantID     int64  `json:"tenantId"`
	StoreID      int64  `json:"storeId"`
	ImportTaskID int64  `json:"importTaskId"`
	Platform     string `json:"platform"`
	Region       string `json:"region"`
	ProductID    string `json:"productId"` // ASIN或产品ID
	CategoryID   int64  `json:"categoryId"`
	RawJsonData  string `json:"rawJsonData"`
	Creator      string `json:"creator"`
}

// RawJsonDataAPI 原始JSON数据API接口定义
type RawJsonDataAPI interface {
	// GetRawJsonData 获取原始JSON数据
	GetRawJsonData(req *RawJsonDataReqDTO) (*RawJsonDataRespDTO, error)

	// CreateRawJsonData 创建原始JSON数据（提交到服务器缓存）
	CreateRawJsonData(req *RawJsonDataCreateReqDTO) (int64, error)
}
