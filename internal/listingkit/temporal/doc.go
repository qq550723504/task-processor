// Package temporal contains ListingKit Temporal orchestration glue for the
// current SHEIN publish proof of concept.
//
// The package intentionally reuses existing ListingKit business logic and
// treats Temporal as the durable orchestration layer, not as a replacement for
// domain rules.
//
// The Temporal SDK dependency is kept in place for this PoC because the next
// task will wire these contracts into real workflow and client usage.
package temporal
