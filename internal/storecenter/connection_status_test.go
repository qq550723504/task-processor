package storecenter

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestConnectionStatusBlankReferenceIsDisconnectedWithoutProviderCall(t *testing.T) {
	provider := &connectionStatusProviderStub{status: ConnectionStatusConnected}
	got := resolveConnectionStatus(context.Background(), provider, ConnectionStatusInput{
		OrganizationID: "org-a", StoreID: uuid.NewString(), Platform: PlatformShein,
	}, time.Second)
	if got != ConnectionStatusDisconnected {
		t.Fatalf("resolveConnectionStatus() = %q, want disconnected", got)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
}

func TestConnectionStatusProviderResultsAreStrictlyProjected(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status ConnectionStatus
		err    error
		want   ConnectionStatus
	}{
		{name: "connected", status: ConnectionStatusConnected, want: ConnectionStatusConnected},
		{name: "expired", status: ConnectionStatusExpired, want: ConnectionStatusExpired},
		{name: "explicit unavailable", status: ConnectionStatusUnavailable, want: ConnectionStatusUnavailable},
		{name: "invalid", status: ConnectionStatus("credential-valid"), want: ConnectionStatusUnavailable},
		{name: "error", err: errors.New("provider token leaked internally"), want: ConnectionStatusUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &connectionStatusProviderStub{status: tc.status, err: tc.err}
			got := resolveConnectionStatus(context.Background(), provider, ConnectionStatusInput{
				OrganizationID: "org-a", StoreID: uuid.NewString(), Platform: PlatformShein, ConnectionRef: "opaque-ref",
			}, time.Second)
			if got != tc.want {
				t.Fatalf("resolveConnectionStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestConnectionStatusTimeoutIsUnavailable(t *testing.T) {
	provider := &connectionStatusProviderStub{waitForCancellation: true}
	started := time.Now()
	got := resolveConnectionStatus(context.Background(), provider, ConnectionStatusInput{
		OrganizationID: "org-a", StoreID: uuid.NewString(), Platform: PlatformShein, ConnectionRef: "opaque-ref",
	}, 10*time.Millisecond)
	if got != ConnectionStatusUnavailable {
		t.Fatalf("resolveConnectionStatus() = %q, want unavailable", got)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("timeout elapsed = %s, want bounded", elapsed)
	}
}

type connectionStatusProviderStub struct {
	mu                  sync.Mutex
	status              ConnectionStatus
	err                 error
	waitForCancellation bool
	calls               int
	active              int
	maxActive           int
}

func (p *connectionStatusProviderStub) Status(ctx context.Context, _ ConnectionStatusInput) (ConnectionStatus, error) {
	p.mu.Lock()
	p.calls++
	p.active++
	if p.active > p.maxActive {
		p.maxActive = p.active
	}
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.active--
		p.mu.Unlock()
	}()
	if p.waitForCancellation {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return p.status, p.err
}
