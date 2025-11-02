package api

// ProductImportTaskRespDTO 产品导入任务响应DTO
type ProductImportTaskRespDTO struct {
	ID            int64  `json:"id"`
	TenantID      int64  `json:"tenantId"`
	StoreID       int64  `json:"storeId"`
	Platform      string `json:"platform"`
	Region        string `json:"region"`
	CategoryID    int64  `json:"categoryId"`
	ProductID     string `json:"productId"` // ASIN或产品ID
	Status        int16  `json:"status"`
	ErrorMessage  string `json:"errorMessage"`
	RetryCount    int    `json:"retryCount"`
	MaxRetryCount int    `json:"maxRetryCount"`
	Remark        string `json:"remark"`
	Priority      int    `json:"priority"`
	CreateTime    int64  `json:"createTime"` // Unix时间戳（毫秒）
	UpdateTime    int64  `json:"updateTime"` // Unix时间戳（毫秒）
	Creator       string `json:"creator"`
	Updater       string `json:"updater"`
}

// ProductImportTaskUpdateReqDTO 产品导入任务更新请求DTO
type ProductImportTaskUpdateReqDTO struct {
	ID           int64  `json:"id"`
	Status       int16  `json:"status"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// ImportTaskAPI 导入任务API接口定义
type ImportTaskAPI interface {
	// GetPendingAndRetryTasks 获取待处理及待重试的任务列表
	// userId 为可选参数，传0表示不过滤
	// storeIds 为店铺编号列表，支持多个店铺编号过滤
	// platform 为平台类型，可选参数，如：amazon、temu、shein等
	GetPendingAndRetryTasks(limit int, userId int64, storeIds []int64, platform string) ([]ProductImportTaskRespDTO, error)

	// UpdateTaskStatus 更新任务状态
	UpdateTaskStatus(req *ProductImportTaskUpdateReqDTO) error
}
