// Package dub integrates Shuomi with Dub's server-side REST API for affiliate
// attribution.
//
// The package deliberately owns only the external adapter boundary:
//   - create/update Dub partners;
//   - create partner referral links;
//   - track lead conversions;
//   - track idempotent sale conversions.
//
// It does not own Shuomi subscriptions, orders, commission accounting, refunds,
// earnings balances, or payouts. Those remain authoritative Shuomi business
// state and may consume Dub attribution results through domain-level ports.
//
// The adapter uses net/http rather than github.com/dubinc/dub-go so the main Go
// module does not import Dub's generated AGPL-3.0-only SDK as a dependency.
package dub
