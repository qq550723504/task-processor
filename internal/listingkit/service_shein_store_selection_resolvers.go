package listingkit

import (
	"context"
)

func (s *service) resolveSheinStoreID(ctx context.Context, task *Task) (int64, error) {
	return buildSubmitRuntimeContextResolver(s).resolveStoreID(ctx, task)
}

func (s *service) resolveSheinStoreProfile(ctx context.Context, task *Task) (*ListingKitStoreProfile, error) {
	return buildSubmitRuntimeContextResolver(s).resolveStoreProfile(ctx, task)
}

func (s *service) resolveSheinStoreSelection(ctx context.Context, task *Task) (*sheinStoreSelection, error) {
	return buildSubmitRuntimeContextResolver(s).resolveStoreSelection(ctx, task)
}
