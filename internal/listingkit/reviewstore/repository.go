package reviewstore

import "context"

type Repository interface {
	SaveReview(ctx context.Context, record *ReviewRecord) error
	ListReviews(ctx context.Context, taskID string) ([]ReviewRecord, error)
}
