package assetinspect

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/commercetool"
)

type freshPrincipalFixture struct {
	calls int
	err   error
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
