package listingadmin

import (
	"context"
	"errors"
	"time"

	"task-processor/internal/pkg/types"
)

var errGormStoreAPICookieProviderNotConfigured = errors.New("store cookie provider is not configured")

// NewGormStoreAPI adapts the store repository for callers that only need
// database-backed store reads and updates. Cookie and pause operations remain
// unavailable because they require a separate Redis-backed dependency.
func NewGormStoreAPI(repository *GormStoreRepository) StoreAPI {
	if repository == nil {
		return nil
	}
	return gormStoreAPI{repository: repository}
}

type gormStoreAPI struct {
	repository *GormStoreRepository
}

func (a gormStoreAPI) GetStore(id int64) (*StoreRespDTO, error) {
	if a.repository == nil {
		return nil, nil
	}
	store, err := a.repository.FindStoreByID(context.Background(), id)
	if errors.Is(err, ErrStoreNotFound) {
		return nil, nil
	}
	if err != nil || store == nil {
		return nil, err
	}
	now := time.Now()
	if store.ValidFrom != nil && now.Before(*store.ValidFrom) {
		return nil, nil
	}
	if store.ValidUntil != nil && now.After(*store.ValidUntil) {
		return nil, nil
	}
	return StoreToRespDTO(store), nil
}

func (a gormStoreAPI) PageStores(req *StorePageReqDTO) (*PageResult[*StoreRespDTO], error) {
	if a.repository == nil {
		return nil, nil
	}
	query := StoreQuery{}
	if req != nil {
		query.TenantID = req.TenantID
		query.Platform = req.Platform
		query.Page = req.PageNo
		query.PageSize = req.PageSize
		query.EnableAutoPrice = req.EnableAutoPrice
	}
	page, err := a.repository.ListStores(context.Background(), query)
	if err != nil || page == nil {
		return nil, err
	}
	items := make([]*StoreRespDTO, 0, len(page.Items))
	for index := range page.Items {
		items = append(items, StoreToRespDTO(&page.Items[index]))
	}
	return &PageResult[*StoreRespDTO]{List: items, Total: page.Total, PageNo: page.Page, PageSize: page.PageSize}, nil
}

func (a gormStoreAPI) GetStoreCookie(int64) (string, error) {
	return "", errGormStoreAPICookieProviderNotConfigured
}

func (a gormStoreAPI) UpdateStoreId(req *StoreIdUpdateReqDTO) (bool, error) {
	if a.repository == nil || req == nil {
		return false, nil
	}
	store, err := a.repository.UpdateStoreID(context.Background(), req.ID, req.StoreID)
	return err == nil && store != nil, err
}

func (a gormStoreAPI) UpdateStoreStatus(req *StoreStatusUpdateReqDTO) (bool, error) {
	if a.repository == nil || req == nil {
		return false, nil
	}
	store, err := a.repository.FindStoreByID(context.Background(), req.ID)
	if err != nil || store == nil {
		return false, err
	}
	updated, err := a.repository.UpdateStoreStatus(context.Background(), store.TenantID, req.ID, req.Status, req.Remark)
	return err == nil && updated != nil, err
}

func (a gormStoreAPI) DeleteStoreCookie(int64) (bool, error) { return false, nil }

func (a gormStoreAPI) SetStorePauseStatus(int64, bool, string) (bool, error) { return false, nil }

func (a gormStoreAPI) GetStorePauseStatus(int64) (bool, error) { return false, nil }

func (a gormStoreAPI) GetStorePauseStatusDetail(int64) (*StorePauseStatusRespDTO, error) {
	return nil, nil
}

// StoreToRespDTO exposes the management API projection for a store.
func StoreToRespDTO(store *Store) *StoreRespDTO {
	if store == nil {
		return nil
	}
	return &StoreRespDTO{
		ID:                       store.ID,
		TenantID:                 store.TenantID,
		OwnerUserID:              store.OwnerUserID,
		StoreID:                  store.StoreID,
		Name:                     store.Name,
		Username:                 store.Username,
		Password:                 store.Password,
		LoginUrl:                 store.LoginURL,
		ShopType:                 store.ShopType,
		Region:                   store.Region,
		Platform:                 store.Platform,
		DailyLimit:               store.DailyLimit,
		DailyLimitType:           store.DailyLimitType,
		FixedStockCount:          store.FixedStockCount,
		SkuGenerateStrategy:      store.SKUGenerateStrategy,
		Prefix:                   store.Prefix,
		Suffix:                   store.Suffix,
		Proxy:                    store.Proxy,
		EnableAutoListing:        store.EnableAutoListing,
		EnableAutoLogin:          store.EnableAutoLogin,
		EnableDraft:              store.EnableDraft,
		EnableAutoPrice:          store.EnableAutoPrice,
		DedicatedQueueEnabled:    store.DedicatedQueueEnabled,
		EnableRebargain:          store.EnableRebargain,
		EnableBrandAuthorization: store.EnableBrandAuthorization,
		AuthorizedBrandCode:      store.AuthorizedBrandCode,
		AuthorizedBrandName:      store.AuthorizedBrandName,
		TemuPriceRejectStrategy:  store.TemuPriceRejectStrategy,
		PriceType:                store.PriceType,
		Remark:                   store.Remark,
		Status:                   store.Status,
		CreateTime:               types.ToFlexibleTime(store.CreateTime),
		Creator:                  store.CreatedBy,
	}
}
