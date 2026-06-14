package studio

import (
	"context"
	"time"
)

type BatchDesignReviewService[Detail any] struct {
	ensureBatchExists func(context.Context, string) error
	replaceReviews    func(context.Context, string, []string, time.Time) error
	loadDetail        func(context.Context, string) (*Detail, error)
	currentTime       func() time.Time
}

type BatchDesignReviewServiceConfig[Detail any] struct {
	EnsureBatchExists func(context.Context, string) error
	ReplaceReviews    func(context.Context, string, []string, time.Time) error
	LoadDetail        func(context.Context, string) (*Detail, error)
	CurrentTime       func() time.Time
}

func NewBatchDesignReviewService[Detail any](config BatchDesignReviewServiceConfig[Detail]) *BatchDesignReviewService[Detail] {
	return &BatchDesignReviewService[Detail]{
		ensureBatchExists: config.EnsureBatchExists,
		replaceReviews:    config.ReplaceReviews,
		loadDetail:        config.LoadDetail,
		currentTime:       config.CurrentTime,
	}
}

func (s *BatchDesignReviewService[Detail]) ApproveDesigns(ctx context.Context, batchID string, designIDs []string) (*Detail, error) {
	if err := s.ensureBatchExists(ctx, batchID); err != nil {
		return nil, err
	}
	if err := s.replaceReviews(ctx, batchID, designIDs, s.currentTime().UTC()); err != nil {
		return nil, err
	}
	return s.loadDetail(ctx, batchID)
}
