package supplychainapp

import (
	"context"
	"errors"
	"strings"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/review"
	"time"
)

type StageReviewReader interface {
	Read(context.Context, review.Scope, string) (review.Record, error)
}
type ReviewProjection struct {
	Facts   preparation.StageFactsReader
	Reviews StageReviewReader
}

func (p ReviewProjection) pending(ctx context.Context, scope collection.Scope, item preparation.OperationItem, productKey string, version uint64) (bool, error) {
	if item.Status == preparation.ItemPending || item.Status == preparation.ItemRunning || item.Status == preparation.ItemUnknown {
		return true, nil
	}
	if item.Status != preparation.ItemReview {
		return false, nil
	}
	if p.Reviews == nil {
		return false, record.ErrUnavailable
	}
	v, err := p.Reviews.Read(ctx, review.Scope{Org: scope.OrganizationID, Actor: scope.ActorID}, item.ResultReference)
	if err != nil {
		return false, record.ErrUnavailable
	}
	if v.ID != item.ResultReference || v.Org != scope.OrganizationID || v.Owner != scope.ActorID || v.Input.ProductKey != productKey || v.Input.BaseVersion != version {
		return false, record.ErrConflict
	}
	return v.State != "rejected", nil
}
func (p ReviewProjection) RequireUploadReady(ctx context.Context, scope collection.Scope, saved record.TargetRecord) error {
	if p.Facts == nil {
		return record.ErrUnavailable
	}
	item, err := p.Facts.LatestOptimization(ctx, scope, saved.Source.ID, saved.Input.StoreID)
	if errors.Is(err, preparation.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if item.RecordID != "" && (item.RecordID != saved.ID || item.RecordRevision != saved.Revision) {
		return nil
	}
	pending, err := p.pending(ctx, scope, item, saved.Source.Source.ProductKey, saved.EffectiveVersion)
	if err != nil {
		return err
	}
	if pending {
		return record.ErrNotReady
	}
	return nil
}

type StageItem struct {
	SourceID    string                     `json:"sourceId"`
	Stage       string                     `json:"stage"`
	OperationID string                     `json:"operationId,omitempty"`
	Review      *preparation.OperationItem `json:"review,omitempty"`
}
type StagePage struct {
	Items      []StageItem      `json:"items"`
	Total      int64            `json:"total"`
	BatchTotal int64            `json:"batchTotal"`
	Counts     map[string]int64 `json:"counts"`
	NextCursor string           `json:"nextCursor,omitempty"`
}

func validStage(v string) bool {
	return v == "all" || v == "waiting" || v == "missing" || v == "ready" || v == "review" || v == "uploaded"
}
func (a *Application) Stages(ctx context.Context, id, storeID, stage string, q collection.Query, sourceKind string) (StagePage, error) {
	out := StagePage{Items: []StageItem{}, Counts: map[string]int64{"all": 0, "waiting": 0, "missing": 0, "ready": 0, "review": 0, "uploaded": 0}}
	if sourceKind != "" && sourceKind != "own" && sourceKind != "acquisition" || a.StageProjection.Facts == nil || !collection.ValidID(id) || !collection.ValidID(storeID) || !validStage(stage) || q.Validate() != nil {
		return out, preparation.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()
	scope, err := a.Authorization.Authorize(ctx, preparation.PermissionRead)
	if err != nil {
		return out, err
	}
	prep, err := a.Preparations.Read(ctx, id)
	if err != nil {
		return out, err
	}
	out.BatchTotal = prep.Count
	merchant, err := a.PublicationStores.RulesMerchant(ctx, scope, storeID, nil)
	if err != nil {
		return out, err
	}
	binding := merchant.Binding()
	if binding.OrganizationID != scope.OrganizationID || binding.StoreID != storeID || binding.Site != "shein-us" {
		return out, record.ErrForbidden
	}
	limit := q.Limit
	if limit == 0 {
		limit = 20
	}
	after := ""
	cursorFound := q.After == ""
	last := ""
	for {
		facts, e := a.StageProjection.Facts.ListStageFacts(ctx, scope, id, storeID, binding.SupplierIdentityHash, after)
		if e != nil {
			return out, e
		}
		for _, f := range facts {
			if f.SourceID <= last {
				return out, record.ErrUnavailable
			}
			last = f.SourceID
			current := StageItem{SourceID: f.SourceID, Stage: "waiting"}
			if f.RecordID != "" {
				current.Stage = "missing"
				if f.Ready {
					current.Stage = "ready"
				}
				if f.PublishedRecordID == f.RecordID {
					current.Stage = "uploaded"
				}
			}
			if f.Optimization.Status != "" && (f.Optimization.RecordID == f.RecordID && f.Optimization.RecordRevision == f.RecordRevision || f.Optimization.RecordID == "" && (f.Optimization.Status == preparation.ItemPending || f.Optimization.Status == preparation.ItemRunning)) {
				pending, e := a.StageProjection.pending(ctx, scope, f.Optimization, f.ProductKey, f.EffectiveVersion)
				if e != nil {
					return out, e
				}
				if pending {
					current.Stage = "review"
					current.OperationID = f.OptimizationOperationID
					if f.Optimization.Status == preparation.ItemReview {
						v := f.Optimization
						current.Review = &v
					}
				}
			}
			out.Counts["all"]++
			out.Counts[current.Stage]++
			if q.After == f.SourceID {
				cursorFound = true
			}
			if !matchesStageFilter(f, current.Stage, stage, q, sourceKind, prep.Name) {
				continue
			}
			out.Total++
			if q.After != "" && f.SourceID <= q.After {
				continue
			}
			if len(out.Items) < limit {
				out.Items = append(out.Items, current)
			} else if out.NextCursor == "" {
				out.NextCursor = out.Items[len(out.Items)-1].SourceID
			}
		}
		if len(facts) < 100 {
			break
		}
		after = facts[len(facts)-1].SourceID
	}
	if !cursorFound {
		return out, preparation.ErrNotFound
	}
	if out.Counts["all"] != prep.Count {
		return out, record.ErrConflict
	}
	fresh, err := a.Authorization.Authorize(ctx, preparation.PermissionRead)
	if err == nil && fresh != scope {
		err = record.ErrForbidden
	}
	return out, err
}

func matchesStageFilter(f preparation.SourceStageFacts, currentStage, stage string, q collection.Query, sourceKind, batch string) bool {
	return (stage == "all" || stage == currentStage) && (sourceKind == "" || f.SourceKind == sourceKind) && (q.Keyword == "" || strings.Contains(strings.ToLower(f.Title+" "+f.ProductKey+" "+f.SKUs+" "+batch), strings.ToLower(q.Keyword)))
}
