package listingkit

import "context"

type taskPreviewDecorationWiring struct {
	decorateSheinCookieAvailabilityPreview func(context.Context, *Task, *ListingKitPreview)
	decorateSheinStoreResolutionPreview    func(context.Context, *Task, *ListingKitPreview)
}

type taskPreviewAccessWiring struct {
	getTaskPreview   func(context.Context, string, string) (*ListingKitPreview, error)
	buildTaskPreview func(context.Context, *Task, string) (*ListingKitPreview, error)
}

func buildTaskPreviewAccessWiring(s *service) taskPreviewAccessWiring {
	return taskPreviewAccessWiring{
		getTaskPreview: func(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {
			return s.GetTaskPreview(ctx, taskID, platform)
		},
		buildTaskPreview: func(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {
			return s.buildTaskPreview(ctx, task, platform)
		},
	}
}

func buildTaskPreviewDecorationWiring(s *service) taskPreviewDecorationWiring {
	return taskPreviewDecorationWiring{
		decorateSheinCookieAvailabilityPreview: func(ctx context.Context, task *Task, preview *ListingKitPreview) {
			s.decorateSheinCookieAvailabilityPreview(ctx, task, preview)
		},
		decorateSheinStoreResolutionPreview: func(ctx context.Context, task *Task, preview *ListingKitPreview) {
			s.decorateSheinStoreResolutionPreview(ctx, task, preview)
		},
	}
}

func buildTaskPreview(ctx context.Context, task *Task, platform string, decorators taskPreviewDecorationWiring) (*ListingKitPreview, error) {
	preview, err := buildListingKitPreview(task, platform)
	if err != nil {
		return nil, err
	}
	if decorators.decorateSheinCookieAvailabilityPreview != nil {
		decorators.decorateSheinCookieAvailabilityPreview(ctx, task, preview)
	}
	return preview, nil
}

func finalizeTaskPreview(ctx context.Context, task *Task, preview *ListingKitPreview, decorators taskPreviewDecorationWiring) error {
	if decorators.decorateSheinStoreResolutionPreview != nil {
		decorators.decorateSheinStoreResolutionPreview(ctx, task, preview)
	}
	return nil
}
