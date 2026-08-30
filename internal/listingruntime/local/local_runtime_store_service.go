package local

import (
	"context"
	"fmt"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
)

func (r *LocalRuntime) GetRuntimeStoreService() listingruntime.StoreService {
	if r == nil || (r.resources == nil && r.provider == nil) {
		return nil
	}
	return localRuntimeStoreService{runtime: r}
}

func (r *LocalRuntime) GetRuntimeStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error) {
	service := r.GetRuntimeStoreService()
	if service == nil {
		return nil, nil
	}
	return service.GetStorePauseStatusDetail(storeID)
}

func (r *LocalRuntime) SetRuntimeStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error) {
	service := r.GetRuntimeStoreService()
	if service == nil {
		return false, nil
	}
	return service.SetStorePauseStatus(storeID, pause, pauseType)
}

type localRuntimeStoreService struct {
	runtime *LocalRuntime
}

func (s localRuntimeStoreService) GetStore(storeID int64) (*listingruntime.StoreInfo, error) {
	if s.runtime == nil {
		return nil, fmt.Errorf("store runtime is not configured")
	}
	repo := s.runtime.GetLocalStoreRepository()
	if repo == nil {
		return nil, fmt.Errorf("store repository is not configured")
	}
	store, err := repo.FindStoreByID(context.Background(), storeID)
	if err != nil {
		return nil, err
	}
	return runtimeStoreFromListing(store), nil
}

func (s localRuntimeStoreService) GetStorePauseStatus(storeID int64) (bool, error) {
	state := s.storeRuntimeState()
	if state == nil {
		return false, nil
	}
	return state.GetStorePauseStatus(storeID)
}

func (s localRuntimeStoreService) GetStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error) {
	state := s.storeRuntimeState()
	if state == nil {
		return nil, nil
	}
	detail, err := state.GetStorePauseStatusDetail(storeID)
	if err != nil {
		return nil, err
	}
	return runtimePauseDetailFromListingAdminDTO(detail), nil
}

func (s localRuntimeStoreService) SetStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error) {
	state := s.storeRuntimeState()
	if state == nil {
		return false, nil
	}
	return state.SetStorePauseStatus(storeID, pause, pauseType)
}

func (s localRuntimeStoreService) storeRuntimeState() *localStoreRuntimeState {
	if s.runtime == nil {
		return nil
	}
	if s.runtime.resources != nil {
		storeAPI := listingadmin.NewGormStoreAPI(s.runtime.GetLocalStoreRepository())
		if state := newLocalStoreRuntimeState(s.runtime.resources, storeAPI); state != nil {
			return state
		}
	}
	if s.runtime.provider != nil {
		return s.runtime.provider.storeRuntimeState()
	}
	return nil
}
