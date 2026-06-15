package authorizedbrand

import "context"

type contextKey struct{}

func WithResolved(ctx context.Context, value *Resolved) context.Context {
	if ctx == nil || value == nil || !value.Enabled {
		return ctx
	}

	return context.WithValue(ctx, contextKey{}, *value)
}

func FromContext(ctx context.Context) (*Resolved, bool) {
	if ctx == nil {
		return nil, false
	}

	value, ok := ctx.Value(contextKey{}).(Resolved)
	if !ok || !value.Enabled {
		return nil, false
	}

	return &value, true
}
