// Package product 提供产品领域类型定义
package product

import (
	"task-processor/internal/infra/clients/management/api"
)

// RawJsonDataClient 原始JSON数据客户端接口
type RawJsonDataClient interface {
	GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error)
	CreateRawJsonData(req *api.RawJsonDataCreateReqDTO) (int64, error)
}

// FetchRequest 获取请求
type FetchRequest struct {
	TenantID   int64
	Platform   string
	Region     string
	ProductID  string
	StoreID    int64
	CategoryID int64
	Creator    string
}
