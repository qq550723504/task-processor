package storecenter

import (
	"context"
	"errors"
	"sync"
	"time"
)

const connectionStatusTimeout = 500 * time.Millisecond

func (s *Service) List(ctx context.Context, request ListStoresRequest) (ListStoresResult, error) {
	normalized, query, err := normalizeListStoresRequest(request)
	if err != nil {
		return ListStoresResult{}, err
	}
	page, err := s.repository.List(ctx, normalized.OrganizationID, query)
	if err != nil {
		return ListStoresResult{}, dependencyError(err)
	}
	if page.Total < 0 || int64(len(page.Stores)) > page.Total || len(page.Stores) > normalized.PageSize {
		return ListStoresResult{}, dependencyError(errors.New("store page is inconsistent"))
	}
	items := make([]StoreProjection, len(page.Stores))
	for i := range page.Stores {
		store, cloneErr := RehydrateStore(page.Stores[i].Snapshot())
		if cloneErr != nil || store.OrganizationID() != normalized.OrganizationID {
			return ListStoresResult{}, dependencyError(cloneErr)
		}
		items[i].Store = *store
	}
	s.projectConnections(ctx, items)
	summary, err := s.quota.Summary(ctx, normalized.OrganizationID)
	if err != nil {
		return ListStoresResult{}, dependencyError(err)
	}
	quota, err := validateQuotaSummary(normalized.OrganizationID, summary)
	if err != nil {
		return ListStoresResult{}, dependencyError(err)
	}
	return ListStoresResult{Items: items, Total: page.Total, Page: normalized.Page, PageSize: normalized.PageSize, Quota: quota}, nil
}

func (s *Service) Get(ctx context.Context, request GetStoreRequest) (StoreProjection, error) {
	normalized, err := normalizeGetStoreRequest(request)
	if err != nil {
		return StoreProjection{}, err
	}
	store, err := s.repository.Get(ctx, normalized.OrganizationID, normalized.StoreID)
	if errors.Is(err, ErrNotFound) {
		return StoreProjection{}, ErrNotFound
	}
	if err != nil {
		return StoreProjection{}, dependencyError(err)
	}
	if store == nil || store.OrganizationID() != normalized.OrganizationID || store.ID() != normalized.StoreID {
		return StoreProjection{}, dependencyError(errors.New("store read identity mismatch"))
	}
	return s.projectOne(ctx, store)
}

func (s *Service) projectOne(ctx context.Context, store *Store) (StoreProjection, error) {
	if store == nil {
		return StoreProjection{}, dependencyError(errors.New("nil store"))
	}
	clone, err := RehydrateStore(store.Snapshot())
	if err != nil {
		return StoreProjection{}, dependencyError(err)
	}
	status := resolveConnectionStatus(ctx, s.connections, ConnectionStatusInput{OrganizationID: clone.OrganizationID(), StoreID: clone.ID(), Platform: clone.Platform(), ConnectionRef: clone.ConnectionRef()}, connectionStatusTimeout)
	return StoreProjection{Store: *clone, ConnectionStatus: status}, nil
}

func (s *Service) projectConnections(ctx context.Context, items []StoreProjection) {
	workers := len(items)
	if workers > 8 {
		workers = 8
	}
	jobs := make(chan int)
	var group sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := range jobs {
				store := &items[index].Store
				items[index].ConnectionStatus = resolveConnectionStatus(ctx, s.connections, ConnectionStatusInput{OrganizationID: store.OrganizationID(), StoreID: store.ID(), Platform: store.Platform(), ConnectionRef: store.ConnectionRef()}, connectionStatusTimeout)
			}
		}()
	}
	for index := range items {
		jobs <- index
	}
	close(jobs)
	group.Wait()
}

func normalizeListStoresRequest(request ListStoresRequest) (ListStoresRequest, StoreListQuery, error) {
	organizationID, err := validateOpaqueIdentity("organization ID", request.OrganizationID, MaxOrganizationIDBytes)
	if err != nil || request.Page < 1 || request.PageSize < 1 || request.PageSize > 100 {
		return ListStoresRequest{}, StoreListQuery{}, errors.New("invalid store list request")
	}
	platform := Platform("")
	if request.Platform != "" {
		platform, err = normalizePlatform(request.Platform)
		if err != nil || string(platform) != request.Platform {
			return ListStoresRequest{}, StoreListQuery{}, errors.New("invalid store list platform")
		}
	}
	if request.Status != "" && !validLifecycleStatus(request.Status) {
		return ListStoresRequest{}, StoreListQuery{}, errors.New("invalid store list status")
	}
	request.OrganizationID = organizationID
	return request, StoreListQuery{Platform: platform, Status: request.Status, Page: request.Page, PageSize: request.PageSize}, nil
}

func normalizeGetStoreRequest(request GetStoreRequest) (GetStoreRequest, error) {
	var err error
	if request.OrganizationID, err = validateOpaqueIdentity("organization ID", request.OrganizationID, MaxOrganizationIDBytes); err != nil {
		return GetStoreRequest{}, err
	}
	if request.StoreID, err = canonicalUUID(request.StoreID); err != nil {
		return GetStoreRequest{}, err
	}
	return request, nil
}
