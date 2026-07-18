package sheinsync

import (
	"context"
	"sync"
)

type costRefreshCoordinator struct {
	mu       sync.Mutex
	scopes   map[string]*costRefreshScope
	logError func(string, error)
}

type costRefreshScope struct {
	ctx     context.Context
	refresh func(context.Context) error
	queued  bool
}

func newCostRefreshCoordinator(logError func(string, error)) *costRefreshCoordinator {
	if logError == nil {
		logError = func(string, error) {}
	}
	return &costRefreshCoordinator{
		scopes:   make(map[string]*costRefreshScope),
		logError: logError,
	}
}

// Schedule runs refresh outside the request path. While a scope is running,
// additional requests collapse into exactly one follow-up run using the latest
// detached context. The refresh callback must read the current persisted state.
func (c *costRefreshCoordinator) Schedule(scopeKey string, ctx context.Context, refresh func(context.Context) error) {
	if c == nil || scopeKey == "" || refresh == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	c.mu.Lock()
	if scope, ok := c.scopes[scopeKey]; ok {
		scope.ctx = ctx
		scope.queued = true
		c.mu.Unlock()
		return
	}
	scope := &costRefreshScope{ctx: ctx, refresh: refresh}
	c.scopes[scopeKey] = scope
	c.mu.Unlock()

	go c.run(scopeKey, scope)
}

func (c *costRefreshCoordinator) run(scopeKey string, scope *costRefreshScope) {
	for {
		c.mu.Lock()
		refreshCtx := scope.ctx
		c.mu.Unlock()
		if err := scope.refresh(refreshCtx); err != nil {
			c.logError(scopeKey, err)
		}

		c.mu.Lock()
		if scope.queued {
			scope.queued = false
			c.mu.Unlock()
			continue
		}
		delete(c.scopes, scopeKey)
		c.mu.Unlock()
		return
	}
}
