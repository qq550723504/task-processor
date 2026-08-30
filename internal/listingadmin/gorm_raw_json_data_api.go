package listingadmin

import (
	"context"

	"task-processor/internal/pkg/types"
)

// NewGormRawJsonDataAPI adapts the Raw JSON repository for callers using the
// management API DTO contract.
func NewGormRawJsonDataAPI(repository *GormRawJSONDataRepository) RawJsonDataAPI {
	if repository == nil {
		return nil
	}
	return gormRawJsonDataAPI{repository: repository}
}

type gormRawJsonDataAPI struct {
	repository *GormRawJSONDataRepository
}

func (a gormRawJsonDataAPI) GetRawJsonData(req *RawJsonDataReqDTO) (*RawJsonDataRespDTO, error) {
	if a.repository == nil || req == nil {
		return nil, nil
	}
	record, err := a.repository.GetLatestRawJSONData(context.Background(), req.Platform, req.ProductID, req.Region)
	if err != nil || record == nil {
		return nil, err
	}
	return RawJSONDataToRespDTO(record), nil
}

func (a gormRawJsonDataAPI) CreateRawJsonData(req *RawJsonDataCreateReqDTO) (int64, error) {
	if a.repository == nil || req == nil {
		return 0, nil
	}
	record, err := a.repository.UpsertRawJSONData(context.Background(), &RawJSONData{
		TenantID:     req.TenantID,
		StoreID:      req.StoreID,
		ImportTaskID: req.ImportTaskID,
		Platform:     req.Platform,
		ProductID:    req.ProductID,
		Region:       req.Region,
		CategoryID:   req.CategoryID,
		RawJSONData:  req.RawJsonData,
		Creator:      req.Creator,
		Updater:      req.Creator,
	})
	if err != nil || record == nil {
		return 0, err
	}
	return record.ID, nil
}

// RawJSONDataToRespDTO exposes the management API projection for raw JSON data.
func RawJSONDataToRespDTO(record *RawJSONData) *RawJsonDataRespDTO {
	if record == nil {
		return nil
	}
	createTime := types.FlexibleTime{}
	if record.CreateTime != nil {
		createTime = types.FlexibleTime{Time: *record.CreateTime}
	}
	updateTime := types.FlexibleTime{}
	if record.UpdateTime != nil {
		updateTime = types.FlexibleTime{Time: *record.UpdateTime}
	}
	return &RawJsonDataRespDTO{
		ID:          record.ID,
		TaskID:      record.ImportTaskID,
		Platform:    record.Platform,
		ProductID:   record.ProductID,
		Region:      record.Region,
		RawJSONData: record.RawJSONData,
		CreateTime:  createTime,
		UpdateTime:  updateTime,
	}
}
