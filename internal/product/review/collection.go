package review

import "context"

const MaxPageSize = 100

type PageCursor struct {
	ID string
}

type PageRequest struct {
	Limit  int
	Cursor *PageCursor
}

func (r PageRequest) Validate() error {
	if r.Limit < 1 || r.Limit > MaxPageSize {
		return ErrInvalid
	}
	if r.Cursor != nil && !validID(r.Cursor.ID) {
		return ErrInvalid
	}
	return nil
}

type CollectionItem struct {
	ID          string
	ProductKey  string
	BaseVersion uint64
	Revision    uint64
	State       string
}

type Page struct {
	Items      []CollectionItem
	NextCursor *PageCursor
}

func (s *Service) List(ctx context.Context, request PageRequest) (Page, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, false, false)
	if err != nil {
		return Page{}, err
	}
	if err := request.Validate(); err != nil {
		return Page{}, err
	}
	return s.store.List(ctx, scope, request)
}

// CollectionItemForPersistence projects a validated bounded persistence row.
func CollectionItemForPersistence(record Record) (CollectionItem, error) {
	if err := ValidateStoredRecord(record); err != nil {
		return CollectionItem{}, err
	}
	if record.State != "pending" && record.State != "accepted" {
		return CollectionItem{}, ErrUnavailable
	}
	item := CollectionItem{ID: record.ID, ProductKey: record.Input.ProductKey, BaseVersion: record.Input.BaseVersion, Revision: record.Revision, State: record.State}
	if err := ValidateCollectionItem(item); err != nil {
		return CollectionItem{}, err
	}
	return item, nil
}

func ValidateCollectionItem(item CollectionItem) error {
	if !validID(item.ID) || !ValidKey(item.ProductKey) || item.BaseVersion == 0 || item.BaseVersion > 1<<63-1 || item.Revision == 0 || item.Revision > 1<<63-1 || (item.State != "pending" && item.State != "accepted") {
		return ErrUnavailable
	}
	return nil
}
