package productimage

import "context"

type capabilityUsageQuoteContextKey struct{}

// WithCapabilityUsageQuote binds a governed invocation to a previously
// authorized provider route. It is intentionally scoped to a single
// capability call.
func WithCapabilityUsageQuote(ctx context.Context, quote CapabilityUsageQuote) context.Context {
	return context.WithValue(ctx, capabilityUsageQuoteContextKey{}, quote)
}

func CapabilityUsageQuoteFromContext(ctx context.Context) (CapabilityUsageQuote, bool) {
	if ctx == nil {
		return CapabilityUsageQuote{}, false
	}
	quote, ok := ctx.Value(capabilityUsageQuoteContextKey{}).(CapabilityUsageQuote)
	return quote, ok
}
