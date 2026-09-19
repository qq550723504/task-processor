package assetinspect

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/commercetool"
	"task-processor/internal/product/catalog"
)

type freshPrincipalFixture struct {
	calls int
	err   error
}

type blockingFreshPrincipalFixture struct{}

func (blockingFreshPrincipalFixture) ResolveFreshPrincipal(ctx context.Context) (commercetool.Principal, error) {
	if _, ok := ctx.Deadline(); !ok {
		return commercetool.Principal{}, errors.New("missing tool deadline")
	}
	<-ctx.Done()
	return commercetool.Principal{}, ctx.Err()
}

func TestInvokerDeadlineCoversFreshPreflight(t *testing.T) {
	for _, callerDeadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "tool-budget", true: "earlier-caller-budget"}[callerDeadline], func(t *testing.T) {
			published, inventory := projectionFixture()
			snapshots := &snapshotFixture{value: published}
			assets := &inventoryFixture{value: inventory}
			invoker, err := NewInvoker(snapshots, assets, blockingFreshPrincipalFixture{}, testAgent(), testDependencies())
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			budget := Definition().Timeout.Duration
			if callerDeadline {
				var cancel context.CancelFunc
				budget = 20 * time.Millisecond
				ctx, cancel = context.WithTimeout(ctx, budget)
				defer cancel()
			}
			start := time.Now()
			_, err = invoker.Invoke(ctx, testMetadata(), Input{ProductKey: "product-1", CatalogVersion: "1", TargetPlatform: "shein"})
			elapsed := time.Since(start)
			if elapsed < budget/2 || elapsed > budget+time.Second || snapshots.calls != 0 || assets.calls != 0 || err == nil {
				t.Fatalf("fresh not covered by budget: elapsed=%s budget=%s error=%v", elapsed, budget, err)
			}
		})
	}
}

type blockingSnapshotFixture struct{ calls int }

func (f *blockingSnapshotFixture) GetSnapshot(ctx context.Context, _ catalog.SnapshotIdentity, _ uint64) (catalog.PublishedSnapshot, error) {
	f.calls++
	<-ctx.Done()
	return catalog.PublishedSnapshot{}, ctx.Err()
}

func TestInvokerCallerCancellationStopsReader(t *testing.T) {
	for _, precanceled := range []bool{false, true} {
		snapshots := &blockingSnapshotFixture{}
		assets := &inventoryFixture{}
		invoker, err := NewInvoker(snapshots, assets, &freshPrincipalFixture{}, testAgent(), testDependencies())
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		if precanceled {
			cancel()
		}
		result, err := invoker.Invoke(ctx, testMetadata(), Input{ProductKey: "product-1", CatalogVersion: "1", TargetPlatform: "shein"})
		cancel()
		wantCalls := 1
		if precanceled {
			wantCalls = 0
		}
		if commercetool.CodeOf(err) != commercetool.ErrorDeadlineExceeded || len(result.Output) != 0 || snapshots.calls != wantCalls || assets.calls != 0 {
			t.Fatalf("canceled reader result=%s err=%v calls=%d assets=%d", result.Output, err, snapshots.calls, assets.calls)
		}
	}
}

func (f *freshPrincipalFixture) ResolveFreshPrincipal(ctx context.Context) (commercetool.Principal, error) {
	f.calls++
	if f.err != nil {
		return commercetool.Principal{}, f.err
	}
	return (principalFixture{}).ResolvePrincipal(ctx)
}

func TestInvokerCannotBypassFreshAuthorizationWithGenericDependencies(t *testing.T) {
	published, inventory := projectionFixture()
	snapshots := &snapshotFixture{value: published}
	assets := &inventoryFixture{value: inventory}
	fresh := &freshPrincipalFixture{}
	// The ordinary dependencies intentionally contain an always-successful
	// cached-style resolver. NewInvoker must replace it with the fresh port.
	agent := testAgent()
	agent.AllowedTools = append(agent.AllowedTools, commercetool.ToolRef{ID: "other.inspect", Version: "v1.0.0"})
	invoker, err := NewInvoker(snapshots, assets, fresh, agent, testDependencies())
	if err != nil {
		t.Fatal(err)
	}
	input := Input{ProductKey: "product-1", CatalogVersion: "9007199254740993", TargetPlatform: "fixture-platform"}
	if _, err := invoker.Invoke(context.Background(), testMetadata(), input); err != nil {
		t.Fatalf("first read: %v", err)
	}
	fresh.err = errors.New("revoked")
	if result, err := invoker.Invoke(context.Background(), testMetadata(), input); err == nil || commercetool.CodeOf(err) != commercetool.ErrorIdentityIntegrity || len(result.Output) != 0 {
		t.Fatalf("fresh error bypassed: %v", err)
	}
	if fresh.calls != 2 || snapshots.calls != 1 || assets.calls != 1 {
		t.Fatalf("calls fresh=%d catalog=%d asset=%d", fresh.calls, snapshots.calls, assets.calls)
	}
}

func TestInvokerRejectsMissingFreshOrInvalidAllowlist(t *testing.T) {
	for _, scenario := range []string{"nil", "typed-nil", "missing", "duplicate"} {
		t.Run(scenario, func(t *testing.T) {
			var fresh FreshPrincipalResolver = &freshPrincipalFixture{}
			agent := testAgent()
			switch scenario {
			case "nil":
				fresh = nil
			case "typed-nil":
				var pointer *freshPrincipalFixture
				fresh = pointer
			case "missing":
				agent.AllowedTools = nil
			case "duplicate":
				agent.AllowedTools = append(agent.AllowedTools, Definition().Ref)
			}
			if _, err := NewInvoker(&snapshotFixture{}, &inventoryFixture{}, fresh, agent, testDependencies()); err == nil {
				t.Fatal("invalid construction accepted")
			}
		})
	}
}
