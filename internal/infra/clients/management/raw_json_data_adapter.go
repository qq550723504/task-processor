package management

import (
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/product"
)

// RawJsonDataAdapter 将 api.RawJsonDataAPI 适配为 domain/product.RawJsonDataClient
type RawJsonDataAdapter struct {
	client api.RawJsonDataAPI
}

// NewRawJsonDataAdapter 创建适配器
func NewRawJsonDataAdapter(client api.RawJsonDataAPI) product.RawJsonDataClient {
	return &RawJsonDataAdapter{client: client}
}

func (a *RawJsonDataAdapter) GetRawJsonData(req *product.RawJsonReq) (*product.RawJsonResp, error) {
	resp, err := a.client.GetRawJsonData(&api.RawJsonDataReqDTO{
		TenantID:   req.TenantID,
		Platform:   req.Platform,
		ProductID:  req.ProductID,
		Region:     req.Region,
		StoreID:    req.StoreID,
		CategoryID: req.CategoryID,
		Creator:    req.Creator,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	return &product.RawJsonResp{
		ID:          resp.ID,
		Platform:    resp.Platform,
		ProductID:   resp.ProductID,
		Region:      resp.Region,
		RawJSONData: resp.RawJSONData,
		CreateTime:  resp.CreateTime,
		UpdateTime:  resp.UpdateTime,
	}, nil
}

func (a *RawJsonDataAdapter) CreateRawJsonData(req *product.RawJsonCreateReq) (int64, error) {
	return a.client.CreateRawJsonData(&api.RawJsonDataCreateReqDTO{
		TenantID:    req.TenantID,
		StoreID:     req.StoreID,
		Platform:    req.Platform,
		Region:      req.Region,
		ProductID:   req.ProductID,
		CategoryID:  req.CategoryID,
		RawJsonData: req.RawJsonData,
		Creator:     req.Creator,
	})
}
