// Package studio provides Studio-related business logic services.
// This package handles studio batch operations, session management,
// media processing, and draft generation for the ListingKit domain.
//
// Services in this package:
//   - taskStudioBatchService - Studio batch generation and management
//   - taskStudioBatchDraftService - Draft creation and updates
//   - taskStudioBatchRunService - Batch run execution
//   - taskStudioBatchRunExecutor - Batch run executor
//   - taskStudioMediaService - Media asset management
//   - taskStudioSessionService - Session lifecycle management
//
// These services are extracted from the root listingkit package to
// improve modularity and reduce coupling.
//
// Usage:
//
//	import "task-processor/internal/listingkit/service/studio"
//
//	// Create studio services through configuration
//	config := studio.Config{...}
//	svc := studio.NewServices(config)
package studio
