package readinessinspect

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"task-processor/internal/commercetool"
	"task-processor/internal/product/catalog"
)

type waitingFresh struct{ entered chan struct{} }

func (w waitingFresh) ResolveFreshPrincipal(ctx context.Context) (commercetool.Principal, error) {
	close(w.entered)
	<-ctx.Done()
	return commercetool.Principal{}, ctx.Err()
}

type waitingProduct struct{ entered chan struct{} }

func (w waitingProduct) GetSnapshot(ctx context.Context, _ catalog.SnapshotIdentity, _ uint64) (catalog.PublishedSnapshot, error) {
	close(w.entered)
	<-ctx.Done()
	return catalog.PublishedSnapshot{}, ctx.Err()
}

func TestInvokerDeadlineCoversWaitingFreshAndExactRead(t *testing.T) {
	for _, stage := range []string{"fresh", "product"} {
		for _, stop := range []string{"caller_cancel", "caller_deadline", "tool_deadline"} {
			t.Run(stage+"/"+stop, func(t *testing.T) {
				t.Parallel()
				entered := make(chan struct{})
				p := &exactProducts{product: productFixture()}
				a := &exactAssets{inventory: inventoryFixture()}
				var product catalog.VersionedSnapshotReader = p
				var fresh FreshPrincipalResolver = &freshPrincipal{org: "org-a"}
				if stage == "fresh" {
					fresh = waitingFresh{entered}
				} else {
					product = waitingProduct{entered}
				}
				i, err := NewInvoker(product, a, fresh, commercetool.AgentDefinition{ID: "test.agent", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{Definition().Ref}}, commercetool.InvocationDependencies{Authorizer: allowRead{}, Recorder: auditSink{}, Tracer: otel.Tracer("deadline"), Now: time.Now, AuditTimeout: time.Second})
				require.NoError(t, err)
				ctx := context.Background()
				cancel := func() {}
				if stop == "caller_cancel" {
					ctx, cancel = context.WithCancel(ctx)
				}
				if stop == "caller_deadline" {
					ctx, cancel = context.WithTimeout(ctx, 150*time.Millisecond)
				}
				defer cancel()
				type response struct {
					result commercetool.Result
					err    error
				}
				finished := make(chan response, 1)
				started := time.Now()
				go func() {
					result, invokeErr := i.Invoke(ctx, commercetool.CallMetadata{CallID: "call", AgentID: "test.agent", AgentVersion: "v1.0.0", AgentRunID: "run", BusinessTaskID: "correlation"}, validInput())
					finished <- response{result, invokeErr}
				}()
				select {
				case <-entered:
				case <-time.After(time.Second):
					t.Fatal("dependency did not start")
				}
				if stop == "caller_cancel" {
					cancel()
				}
				select {
				case response := <-finished:
					want := commercetool.ErrorDeadlineExceeded
					if stage == "fresh" {
						want = commercetool.ErrorIdentityIntegrity
					}
					require.Equal(t, want, commercetool.CodeOf(response.err))
					require.Empty(t, response.result.Output)
				case <-time.After(5 * time.Second):
					t.Fatal("Tool deadline did not reach dependency")
				}
				if stop == "tool_deadline" {
					require.GreaterOrEqual(t, time.Since(started), Definition().Timeout.Duration)
				} else {
					require.Less(t, time.Since(started), 2*time.Second)
				}
				require.Zero(t, a.calls, "cancel must not reach Asset/compute")
				if stage == "fresh" {
					require.Zero(t, p.calls)
				}
			})
		}
	}
}
