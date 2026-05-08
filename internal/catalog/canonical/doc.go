// Package canonical defines the platform-neutral product contract shared by
// enrichment, ListingKit workflows, SDS adapters, and marketplace publishers.
//
// This package should stay free of product-enrichment runtime dependencies,
// platform API clients, and persistence concerns. Source traces live here so
// downstream publishing packages can distinguish scraped, user-provided,
// derived, and LLM-generated facts before submission.
package canonical
