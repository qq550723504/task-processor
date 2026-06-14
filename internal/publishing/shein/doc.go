// Package shein builds ListingKit SHEIN publishing packages from canonical
// products and processed image assets.
//
// This package owns the canonical-to-SHEIN draft contract. Low-level SHEIN API
// calls stay in internal/shein/api, and legacy runtime flows stay outside this
// package. It also remains the compatibility/model layer for SHEIN submission
// state helpers that interpret submission responses, including action-aware
// acceptance rules, derive remote lookup identities, and own normalized
// submission report/record queries, mutations, event history updates,
// event DTO construction, confirm-remote update construction/application,
// active-attempt checks, and remote-recovery checks used by higher-level
// ListingKit adapters.
package shein
