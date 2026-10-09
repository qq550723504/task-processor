package notificationcenter

import "context"

// NewReaderSource repeats the original authorized, bounded keyset reader for
// details. A source may supply a faster pure Read implementation separately.
func NewReaderSource(name string, personal bool, list func(context.Context, Scope, string, int) (SourcePage, error)) ReaderSource {
	source := ReaderSource{SourceName: name, IsPersonal: personal, List: list}
	source.Batch = func(ctx context.Context, scope Scope, refs []Ref) ([]Item, error) {
		wanted := map[string]bool{}
		for _, ref := range refs {
			wanted[RefKey(ref)] = true
		}
		items := []Item{}
		after, count := "", 0
		seen := map[string]bool{}
		for {
			page, err := list(ctx, scope, after, 100)
			if err != nil {
				return nil, err
			}
			count += len(page.Items)
			if count > MaxItems || len(page.Items) > 100 {
				return nil, ErrCapacity
			}
			for _, item := range page.Items {
				if wanted[RefKey(item.Ref)] {
					items = append(items, item)
				}
			}
			if page.Next == "" {
				break
			}
			if seen[page.Next] || page.Next == after {
				return nil, ErrUnavailable
			}
			seen[page.Next], after = true, page.Next
		}
		if len(items) != len(wanted) {
			return nil, ErrStale
		}
		return items, nil
	}
	source.Read = func(ctx context.Context, scope Scope, ref Ref) (Item, error) {
		after := ""
		seen := map[string]bool{}
		count := 0
		for {
			page, e := list(ctx, scope, after, 100)
			if e != nil {
				return Item{}, e
			}
			count += len(page.Items)
			if count > MaxItems || len(page.Items) > 100 {
				return Item{}, ErrCapacity
			}
			for _, item := range page.Items {
				if item.Ref.EntityID == ref.EntityID && item.Ref.Type == ref.Type {
					return item, nil
				}
			}
			if page.Next == "" {
				return Item{}, ErrNotFound
			}
			if seen[page.Next] || page.Next == after {
				return Item{}, ErrUnavailable
			}
			seen[page.Next] = true
			after = page.Next
		}
	}
	return source
}

// ReaderSource adapts a pure, freshly authorized owner reader. It has no
// database/provider access and cannot execute a business command.
type ReaderSource struct {
	SourceName string
	IsPersonal bool
	List       func(context.Context, Scope, string, int) (SourcePage, error)
	Read       func(context.Context, Scope, Ref) (Item, error)
	Batch      func(context.Context, Scope, []Ref) ([]Item, error)
	// Unopened is reserved for the explicit product backlog engines.
	Unopened bool
}

func (s ReaderSource) Name() string     { return s.SourceName }
func (s ReaderSource) Category() string { return Business }
func (s ReaderSource) Personal() bool   { return s.IsPersonal }
func (s ReaderSource) ListVisible(ctx context.Context, scope Scope, after string, limit int) (SourcePage, error) {
	if s.List == nil {
		if s.Unopened {
			return SourcePage{}, ErrDependencyMissing
		}
		return SourcePage{}, ErrUnavailable
	}
	return s.List(ctx, scope, after, limit)
}
func (s ReaderSource) ReadVisible(ctx context.Context, scope Scope, ref Ref) (Item, error) {
	if s.Read == nil {
		return Item{}, ErrUnavailable
	}
	return s.Read(ctx, scope, ref)
}
func (s ReaderSource) ReadVisibleRefs(ctx context.Context, scope Scope, refs []Ref) ([]Item, error) {
	if s.Batch != nil {
		return s.Batch(ctx, scope, refs)
	}
	items := make([]Item, 0, len(refs))
	for _, ref := range refs {
		item, err := s.ReadVisible(ctx, scope, ref)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
