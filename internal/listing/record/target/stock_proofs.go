package target

import (
	"context"
	"golang.org/x/sync/errgroup"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
)

type TargetStockProofReader interface {
	ProbeStockProof(context.Context, int, model.StockProof) (goods.OfficialStockProofObservation, error)
}

func ProbeTargetStockProofs(ctx context.Context, images TargetImageReader, draft goods.OfficialDraftInput) ([]goods.OfficialStockProofObservation, error) {
	var selected []goods.OfficialStockProofObservation
	if len(draft.Product.SKCs) > 40 {
		return nil, ErrTooLarge
	}
	for k, skc := range draft.Product.SKCs {
		if len(skc.StockProofs) > 1 {
			return nil, ErrNotReady
		}
		for _, p := range skc.StockProofs {
			if !goods.ValidStockProofInput(p) {
				return nil, ErrNotReady
			}
			selected = append(selected, goods.OfficialStockProofObservation{SKC: k, Filename: p.Filename, Type: p.Type, SourceURL: p.URL})
		}
	}
	if len(selected) == 0 {
		return nil, nil
	}
	reader, ok := images.(TargetStockProofReader)
	if !ok || ctx == nil {
		return nil, ErrUnavailable
	}
	group, bounded := errgroup.WithContext(ctx)
	group.SetLimit(4)
	for i, wanted := range selected {
		group.Go(func() error {
			v, e := reader.ProbeStockProof(bounded, wanted.SKC, model.StockProof{Filename: wanted.Filename, Type: wanted.Type, URL: wanted.SourceURL})
			if e != nil {
				return e
			}
			if v.SKC != wanted.SKC || v.Filename != wanted.Filename || v.Type != wanted.Type || v.SourceURL != wanted.SourceURL || !goods.ValidStockProofObservation(v) {
				return ErrNotReady
			}
			selected[i] = v
			return nil
		})
	}
	if e := group.Wait(); e != nil {
		return nil, e
	}
	return selected, nil
}
