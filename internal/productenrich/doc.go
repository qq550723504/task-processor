// Package productenrich owns product enrichment tasks, LLM generation, source
// scraping adapters, and the compatibility API for the historical product JSON
// workflow.
//
// The platform-neutral canonical product model lives in internal/catalog/canonical.
// This package intentionally keeps aliases for persisted task JSON and older
// callers while new cross-domain code should import canonical directly.
package productenrich
