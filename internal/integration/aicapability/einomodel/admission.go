package einomodel

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
	"task-processor/internal/aicapability"
)

// BoundedAdmission preserves the current text client's per-credential limits
// for both Eino consumers. One instance is shared by Chat and Product Agent
// within the application; no request is retried or queued after its deadline.
type BoundedAdmission struct {
	mu     sync.Mutex
	routes map[admissionKey]*routeBudget
}

type admissionKey struct {
	organizationID string
	clientName     string
}

type routeBudget struct {
	slots chan struct{}
	rate  *rate.Limiter
}

func NewBoundedAdmission() *BoundedAdmission {
	return &BoundedAdmission{routes: make(map[admissionKey]*routeBudget)}
}

// Acquire waits under the caller's existing deadline. The fixed bounds match
// the active text client's 10 concurrent calls, 5 calls/second and burst 15.
// The caller holds the returned slot through the one provider attempt.
func (a *BoundedAdmission) Acquire(ctx context.Context, input aicapability.TextInputIdentity) (func(), error) {
	if a == nil || ctx == nil || ctx.Err() != nil || input.OrganizationID == "" || input.Profile.ClientName == "" {
		return nil, ErrNotDispatched
	}
	key := admissionKey{organizationID: input.OrganizationID, clientName: input.Profile.ClientName}
	a.mu.Lock()
	budget := a.routes[key]
	if budget == nil {
		budget = &routeBudget{slots: make(chan struct{}, 10), rate: rate.NewLimiter(rate.Limit(5), 15)}
		a.routes[key] = budget
	}
	a.mu.Unlock()
	select {
	case budget.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ErrNotDispatched
	}
	if err := budget.rate.Wait(ctx); err != nil {
		<-budget.slots
		return nil, ErrNotDispatched
	}
	return func() { <-budget.slots }, nil
}
