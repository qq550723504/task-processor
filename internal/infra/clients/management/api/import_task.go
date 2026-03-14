package api

// ProductImportTaskRespDTO 产品导入任务响应DTO
type ProductImportTaskRespDTO struct {
	ID            int64  `json:"id"`
	TenantID      int64  `json:"tenantId"`
	StoreID       int64  `json:"storeId"`
	Platform      string `json:"platform"`
	Region        string `json:"region"`
	CategoryID    int64  `json:"categoryId"`
	ProductID     string `json:"productId"`
	Status        int16  `json:"status"`
	ErrorMessage  string `json:"errorMessage"`
	RetryCount    int    `json:"retryCount"`
	MaxRetryCount int    `json:"maxRetryCount"`
	Remark        string `json:"remark"`
	Priority      int    `json:"priority"`
	CreateTime    int64  `json:"createTime"`
	UpdateTime    int64  `json:"updateTime"`
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
	GetPendingAndRetryTasks(limit int, userId int64, storeIds []int64) ([]ProductImportTaskRespDTO, error)
	UpdateTaskStatus(req *ProductImportTaskUpdateReqDTO) error
}
