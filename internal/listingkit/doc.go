// Package listingkit exposes the ListingKit domain model and compatibility
// facade used by HTTP handlers, stores, and platform workflows.
//
// New code should keep orchestration, generation review, workspace editing,
// and submission behavior behind narrower subdomain packages. The root package
// remains the stable API surface for existing routes and persisted task JSON.
package listingkit
