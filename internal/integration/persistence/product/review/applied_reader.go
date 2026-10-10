package reviewpersistence

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/product/review"
)

// NewAppliedPublicationReader exposes existing exact owner-scoped Apply facts,
// including inside a caller-owned Product transaction. It grants no mutation,
// administrator scope, latest-version selection or source publication access.
func NewAppliedPublicationReader(db *gorm.DB) (review.AppliedPublicationLookup, error) {
	if db == nil || db.Dialector.Name() != "postgres" {
		return nil, review.ErrUnavailable
	}
	return appliedReader{db}, nil
}

type appliedReader struct{ db *gorm.DB }

func (r appliedReader) ReadAppliedPublication(ctx context.Context, scope review.Scope, product string, version uint64, publication string) (review.Record, error) {
	if scope.Admin {
		return review.Record{}, review.ErrForbidden
	}
	return readAppliedPublication(ctx, r.db, scope, product, version, publication)
}
