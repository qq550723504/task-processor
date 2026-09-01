package local

import (
	api "task-processor/internal/listingadmin"
	"task-processor/internal/marketplace/sourceproduct"
)

// RawJsonDataAdapter 将 api.RawJsonDataAPI 适配为 domain/sourceproduct.RawJsonDataClient
type RawJsonDataAdapter struct {
	client api.RawJsonDataAPI
}

// NewRawJsonDataAdapter 创建适配器
func NewRawJsonDataAdapter(client api.RawJsonDataAPI) sourceproduct.RawJsonDataClient {
	return &RawJsonDataAdapter{client: client}
}

func (a *RawJsonDataAdapter) GetRawJsonData(req *sourceproduct.RawJsonReq) (*sourceproduct.RawJsonResp, error) {
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
	return &sourceproduct.RawJsonResp{
		ID:          resp.ID,
		Platform:    resp.Platform,
		ProductID:   resp.ProductID,
		Region:      resp.Region,
		RawJSONData: resp.RawJSONData,
		CreateTime:  resp.CreateTime.UnixMilli(),
		UpdateTime:  resp.UpdateTime.UnixMilli(),
	}, nil
}

func (a *RawJsonDataAdapter) GetRawJsonDataAnyFreshness(req *sourceproduct.RawJsonReq) (*sourceproduct.RawJsonResp, error) {
	freshClient, ok := a.client.(interface {
		GetRawJsonDataAnyFreshness(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error)
	})
	if !ok {
		return nil, nil
	}

	resp, err := freshClient.GetRawJsonDataAnyFreshness(&api.RawJsonDataReqDTO{
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
	return &sourceproduct.RawJsonResp{
		ID:          resp.ID,
		Platform:    resp.Platform,
		ProductID:   resp.ProductID,
		Region:      resp.Region,
		RawJSONData: resp.RawJSONData,
		CreateTime:  resp.CreateTime.UnixMilli(),
		UpdateTime:  resp.UpdateTime.UnixMilli(),
	}, nil
}

func (a *RawJsonDataAdapter) CreateRawJsonData(req *sourceproduct.RawJsonCreateReq) (int64, error) {
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
