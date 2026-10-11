package supplymarket

import (
	"context"
	"golang.org/x/sync/errgroup"
	"task-processor/internal/product/collection"
)

type ApplicationCounts struct {
	Reviewing          int64 `json:"reviewing"`
	SupplementRequired int64 `json:"supplementRequired"`
	Approved           int64 `json:"approved"`
}
type ApplicationProduct struct {
	ItemID       string `json:"itemId"`
	Title        string `json:"title"`
	ThumbnailURL string `json:"thumbnailUrl"`
	GroupName    string `json:"groupName"`
	Source       string `json:"source"`
}
type ApplicationOverview struct {
	ApplicationCounts
	EligibleProducts int64                `json:"eligibleProducts"`
	Products         []ApplicationProduct `json:"products"`
}

const overviewScanBudget = 2000

type overviewCollections interface {
	ListItems(context.Context, string, collection.Query) (collection.Page[collection.Item], error)
	ReadBatch(context.Context, string) (collection.Batch, error)
}

func (s *Service) overviewScope(ctx context.Context) (collection.Scope, error) {
	var scope collection.Scope
	for i, p := range []string{PermissionRead, PermissionApply, collection.PermissionRead, collection.PermissionManage} {
		current, err := s.auth.Authorize(ctx, p)
		if err != nil || current.Validate() != nil || i > 0 && current != scope {
			return collection.Scope{}, ErrForbidden
		}
		scope = current
	}
	return scope, ctx.Err()
}

// ApplicationOverview is a request-local projection. Every candidate goes
// through the existing sealed Choice/Apply lineage; it grants no write intent.
func (s *Service) ApplicationOverview(ctx context.Context) (ApplicationOverview, error) {
	empty := ApplicationOverview{}
	if ctx == nil || s == nil {
		return empty, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.overviewScope(ctx)
	if err != nil {
		return empty, err
	}
	collections, ok := s.selected.(overviewCollections)
	if !ok {
		return empty, ErrUnavailable
	}
	counts, err := s.repository.CountApplications(ctx, scope)
	if err != nil {
		return empty, err
	}
	result := ApplicationOverview{ApplicationCounts: counts, Products: []ApplicationProduct{}}
	seen := map[string]bool{}
	groups := map[string]string{}
	after := ""
	expectedTotal := int64(-1)
	for {
		page, err := collections.ListItems(ctx, "", collection.Query{After: after, Limit: 50})
		if err != nil {
			return empty, err
		}
		if expectedTotal < 0 {
			expectedTotal = page.Total
		}
		if page.Total != expectedTotal || page.Total < 0 || page.Total > overviewScanBudget || len(page.Items) > 50 || len(seen)+len(page.Items) > overviewScanBudget {
			return empty, ErrUnavailable
		}
		for _, item := range page.Items {
			if !collection.ValidID(item.ID) || !collection.ValidID(item.BatchID) || item.Source.Kind != "own" || item.ArchivedAt != nil || seen[item.ID] {
				return empty, ErrUnavailable
			}
			seen[item.ID] = true
		}
		type candidate struct {
			optimized    bool
			title, image string
		}
		candidates := make([]candidate, len(page.Items))
		group, work := errgroup.WithContext(ctx)
		group.SetLimit(4)
		for i, item := range page.Items {
			group.Go(func() error {
				choice, e := s.Choice(work, item.ID)
				if e != nil {
					return e
				}
				if choice.Selection.ItemID != item.ID || choice.Selection.ExpectedRevision != item.Revision {
					return ErrConflict
				}
				if choice.Optimized {
					if len(choice.Product.Images) == 0 {
						return ErrUnavailable
					}
					candidates[i] = candidate{true, choice.Product.Title, choice.Product.Images[0]}
				}
				return nil
			})
		}
		if err = group.Wait(); err != nil {
			return empty, err
		}
		for i, c := range candidates {
			if !c.optimized {
				continue
			}
			result.EligibleProducts++
			if len(result.Products) >= 3 {
				continue
			}
			item := page.Items[i]
			name, found := groups[item.BatchID]
			if !found {
				batch, e := collections.ReadBatch(ctx, item.BatchID)
				if e != nil {
					return empty, e
				}
				name = batch.Name
				groups[item.BatchID] = name
			}
			result.Products = append(result.Products, ApplicationProduct{ItemID: item.ID, Title: c.title, ThumbnailURL: c.image, GroupName: name, Source: "own"})
		}
		if page.NextCursor == "" {
			break
		}
		if len(page.Items) == 0 || page.NextCursor != page.Items[len(page.Items)-1].ID || page.NextCursor == after {
			return empty, ErrUnavailable
		}
		after = page.NextCursor
	}
	if int64(len(seen)) != expectedTotal {
		return empty, ErrUnavailable
	}
	current, err := s.overviewScope(ctx)
	if err != nil || current != scope {
		return empty, ErrForbidden
	}
	if ctx.Err() != nil {
		return empty, ctx.Err()
	}
	return result, nil
}
