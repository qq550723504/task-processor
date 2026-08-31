package storecenter

import "context"

// Repository is the Store Center persistence boundary. Every operation is
// explicitly scoped by the effective Organization ID.
type Repository interface {
	CreateOrReplay(ctx context.Context, organizationID string, store *Store) (stored *Store, replayed bool, err error)
	List(ctx context.Context, organizationID string, query StoreListQuery) (StorePage, error)
	Get(ctx context.Context, organizationID string, storeID string) (*Store, error)
	Save(ctx context.Context, organizationID string, store *Store, expectedVersion int64) error
	SoftDelete(ctx context.Context, organizationID string, storeID string, expectedVersion int64) error
}

type StoreListQuery struct {
	Page     int
	PageSize int
	Platform Platform
	Status   StoreStatus
}

type StorePage struct {
	Stores []Store
	Total  int64
}
