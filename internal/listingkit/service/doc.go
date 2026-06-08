// Package service provides the business logic implementation for ListingKit.
// Services orchestrate domain operations, coordinate with repositories,
// and expose high-level APIs for the API layer.
//
// Service organization by domain:
//   - generation_service.go - Generation queue and action management
//   - submission_service.go - Submission readiness and execution
//   - revision_service.go - Revision history and restoration
//   - studio_*.go - Studio session and batch management
//
// Each service depends on:
//   - core/ package for interfaces and models (minimal dependency)
//   - store/ package for persistence
//   - External services (productimage, sds, etc.)
//
// Usage:
//
//	import "task-processor/internal/listingkit/service"
//
//	// Access services through the main Service struct
//	var svc *service.Service
//	svc.Generation.CreateTask(...)
package service
