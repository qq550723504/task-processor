package listingadmin

import "context"

// NewGormInventoryRecordAPI adapts the inventory record repository for callers
// using the management API DTO contract.
func NewGormInventoryRecordAPI(repository *GormInventoryRecordRepository) InventoryRecordAPI {
	if repository == nil {
		return nil
	}
	return gormInventoryRecordAPI{repository: repository}
}

type gormInventoryRecordAPI struct {
	repository *GormInventoryRecordRepository
}

func (a gormInventoryRecordAPI) CreateInventoryRecord(req *InventoryRecordCreateReqDTO) (int64, error) {
	if a.repository == nil || req == nil {
		return 0, nil
	}
	record, err := a.repository.CreateInventoryRecord(context.Background(), &InventoryRecord{
		Platform:           req.Platform,
		ProductID:          req.ProductId,
		Region:             req.Region,
		Stock:              req.Stock,
		StockStatus:        req.StockStatus,
		IsAvailable:        req.IsAvailable,
		OriginalPrice:      req.OriginalPrice,
		CurrentPrice:       req.CurrentPrice,
		Currency:           req.Currency,
		PriceChangePercent: req.PriceChangePercent,
		SyncSource:         req.SyncSource,
		Remark:             req.Remark,
	})
	if err != nil || record == nil {
		return 0, err
	}
	return record.ID, nil
}

func (a gormInventoryRecordAPI) GetLatestInventoryRecord(platform, productID, region string) (*InventoryRecordRespDTO, error) {
	if a.repository == nil {
		return nil, nil
	}
	record, err := a.repository.GetLatestInventoryRecord(context.Background(), platform, productID, region)
	if err != nil || record == nil {
		return nil, err
	}
	return InventoryRecordToRespDTO(record), nil
}

// InventoryRecordToRespDTO exposes the management API projection for an inventory record.
func InventoryRecordToRespDTO(record *InventoryRecord) *InventoryRecordRespDTO {
	if record == nil {
		return nil
	}
	return &InventoryRecordRespDTO{
		ID:                 record.ID,
		Platform:           record.Platform,
		ProductId:          record.ProductID,
		Region:             record.Region,
		Stock:              record.Stock,
		StockStatus:        record.StockStatus,
		IsAvailable:        record.IsAvailable,
		OriginalPrice:      record.OriginalPrice,
		CurrentPrice:       record.CurrentPrice,
		Currency:           record.Currency,
		PriceChangePercent: record.PriceChangePercent,
		SyncSource:         record.SyncSource,
		Remark:             record.Remark,
		CreateTime:         flexibleTimeValue(record.CreateTime),
	}
}
