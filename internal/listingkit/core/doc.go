// Package core provides the fundamental interfaces, models, and utilities
// for the ListingKit domain. This package contains the essential building
// blocks that other listingkit sub-packages depend on.
//
// Core responsibilities:
//   - Define core interfaces (Repository, Service, Processor)
//   - Define core data models (Task, Request, Result)
//   - Provide utility functions (assemblers, helpers)
//
// This package should have minimal dependencies on other listingkit sub-packages.
// It serves as the foundation for the entire ListingKit module.
//
// Usage:
//
//	import "task-processor/internal/listingkit/core"
//
//	// Use core types
//	var task *core.Task
//	var repo core.Repository
package core
