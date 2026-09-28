package browser

import "time"

// newTestClient builds a client with the rate floor and challenge cooldown
// effectively disabled. The production defaults are deliberately slow, so tests
// opt out explicitly instead of inheriting a pace that would make the suite
// meaningless. Throttle behaviour itself is covered by throttle_test.go.
func newTestClient(opts Options) *Client {
	opts.MinInterval = time.Nanosecond
	opts.ChallengeCooldown = time.Nanosecond
	opts.StartupQuarantine = -1
	return New(opts)
}
