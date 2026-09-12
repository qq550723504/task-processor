package assetinspect

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

type snapshotFixture struct {
	value    catalog.PublishedSnapshot
	err      error
	calls    int
	identity catalog.SnapshotIdentity
	version  uint64
}

func (f *snapshotFixture) GetSnapshot(_ context.Context, id catalog.SnapshotIdentity, version uint64) (catalog.PublishedSnapshot, error) {
	f.calls++
	f.identity = id
	f.version = version
	return f.value, f.err
}

type inventoryFixture struct {
	value asset.ApprovedAssetInventory
	err   error
	calls int
	scope asset.InventoryScope
}

func (f *inventoryFixture) GetApprovedInventory(_ context.Context, scope asset.InventoryScope) (asset.ApprovedAssetInventory, error) {
	f.calls++
	f.scope = scope
	return f.value, f.err
}

type principalFixture struct{}

func (principalFixture) ResolvePrincipal(context.Context) (commercetool.Principal, error) {
	return commercetool.Principal{TenantID: "org-a", UserID: "actor", Roles: []string{"read-only-fixture"}}, nil
}

type readOnlyAuthorizer struct{}

func (readOnlyAuthorizer) Authorize(_ context.Context, _ commercetool.Principal, p commercetool.PermissionRequirement) error {
	if p.Permission != authz.PermissionListingKitAdminRead {
		return errors.New("only read allowed")
	}
	return nil
}

type auditFixture struct{}

func (auditFixture) RecordToolCall(context.Context, commercetool.AuditRecord) error { return nil }
func testDependencies() commercetool.InvocationDependencies {
	return commercetool.InvocationDependencies{PrincipalResolver: principalFixture{}, Authorizer: readOnlyAuthorizer{}, Recorder: auditFixture{}, Tracer: otel.Tracer("assetinspect"), Now: time.Now, AuditTimeout: time.Second}
}
func testAgent() commercetool.AgentDefinition {
	return commercetool.AgentDefinition{ID: "test.agent", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{Definition().Ref}}
}
func testMetadata() commercetool.CallMetadata {
	return commercetool.CallMetadata{CallID: "call", AgentID: "test.agent", AgentVersion: "v1.0.0", AgentRunID: "run", BusinessTaskID: "correlation-only"}
}
func testBound(t *testing.T, e commercetool.Executor) *commercetool.BoundToolSet {
	t.Helper()
	r, err := commercetool.NewRegistry(commercetool.Tool{Definition: Definition(), Executor: e})
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Bind(testAgent(), testDependencies())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

const validArguments = `{"product_key":"product-1","catalog_version":"9007199254740993","target_platform":"fixture-platform"}`

func TestExecutorExactReadersAndErrorTaxonomy(t *testing.T) {
	for _, scenario := range []string{"exact", "snapshot-missing", "asset-missing", "snapshot-unavailable", "asset-unavailable", "snapshot-large", "asset-large", "corrupt-binding", "corrupt-payload", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			published, inventory := projectionFixture()
			snapshots := &snapshotFixture{value: published}
			assets := &inventoryFixture{value: inventory}
			want := commercetool.ErrorCode("")
			switch scenario {
			case "snapshot-missing":
				snapshots.err = catalog.ErrSnapshotNotReady
				want = commercetool.ErrorNotFound
			case "asset-missing":
				assets.err = asset.ErrApprovedAssetsNotReady
				want = commercetool.ErrorNotFound
			case "snapshot-unavailable":
				snapshots.err = catalog.ErrRepositoryUnavailable
				want = commercetool.ErrorDependencyUnavailable
			case "asset-unavailable":
				assets.err = asset.ErrRepositoryUnavailable
				want = commercetool.ErrorDependencyUnavailable
			case "snapshot-large":
				snapshots.err = catalog.ErrSnapshotTooLarge
				want = commercetool.ErrorFailedPrecondition
			case "asset-large":
				assets.err = asset.ErrInventoryTooLarge
				want = commercetool.ErrorFailedPrecondition
			case "corrupt-binding":
				assets.value.Scope.SourceSnapshotVersion--
				want = commercetool.ErrorInternal
			case "corrupt-payload":
				assets.err = asset.ErrRepositoryStateInvalid
				want = commercetool.ErrorInternal
			case "canceled":
				assets.err = context.Canceled
				want = commercetool.ErrorDeadlineExceeded
			}
			e, err := NewExecutor(snapshots, assets)
			if err != nil {
				t.Fatal(err)
			}
			result, err := testBound(t, e).Invoke(context.Background(), commercetool.Call{Tool: Definition().Ref, Metadata: testMetadata(), Arguments: json.RawMessage(validArguments)})
			if want == "" {
				if err != nil || !strings.Contains(string(result.Output), `"id":"asset-1"`) {
					t.Fatalf("exact read: %s %v", result.Output, err)
				}
			} else if err == nil || commercetool.CodeOf(err) != want || len(result.Output) != 0 {
				t.Fatalf("want %s got %v output=%s", want, err, result.Output)
			}
			if snapshots.calls != 1 || snapshots.identity != published.Identity || snapshots.version != published.Version {
				t.Fatalf("not exact catalog: %+v", snapshots)
			}
			wantCalls := 1
			if snapshots.err != nil {
				wantCalls = 0
			}
			if assets.calls != wantCalls {
				t.Fatalf("asset calls %d", assets.calls)
			}
			if assets.calls > 0 && assets.scope != inventory.Scope {
				t.Fatalf("not exact platform/version: %+v", assets.scope)
			}
		})
	}
}

func TestExecutorRejectsInvalidInputBeforeReading(t *testing.T) {
	for _, raw := range []string{
		`{"product_key":"product-1","catalog_version":"1"}`,
		`{"product_key":"product-1","catalog_version":"1","target_platform":""}`,
		`{"product_key":"product-1","catalog_version":"1","target_platform":" shein"}`,
		`{"product_key":" product-1","catalog_version":"1","target_platform":"shein"}`,
		`{"product_key":"product-1","catalog_version":"0","target_platform":"shein"}`,
		`{"product_key":"product-1","catalog_version":"01","target_platform":"shein"}`,
		`{"product_key":"product-1","catalog_version":"9223372036854775808","target_platform":"shein"}`,
		`{"product_key":"product-1","catalog_version":"1","target_platform":"shein","task_id":"legacy"}`,
		`{"product_key":"product-1","catalog_version":"1","target_platform":"shein","tenant_id":"other"}`,
		strings.Repeat("x", commercetool.MaxInvocationArgumentsBytes+1),
	} {
		snapshots := &snapshotFixture{}
		assets := &inventoryFixture{}
		e, _ := NewExecutor(snapshots, assets)
		_, err := testBound(t, e).Invoke(context.Background(), commercetool.Call{Tool: Definition().Ref, Metadata: testMetadata(), Arguments: json.RawMessage(raw)})
		if err == nil || commercetool.CodeOf(err) != commercetool.ErrorInvalidInput || snapshots.calls != 0 || assets.calls != 0 {
			t.Fatalf("invalid request reached owner: %v snapshot=%d assets=%d", err, snapshots.calls, assets.calls)
		}
	}
}

func TestExecutorRejectsNilPorts(t *testing.T) {
	var snapshots *snapshotFixture
	var assets *inventoryFixture
	for _, ports := range []struct {
		s catalog.VersionedSnapshotReader
		a asset.ApprovedInventoryReader
	}{{nil, &inventoryFixture{}}, {&snapshotFixture{}, nil}, {snapshots, &inventoryFixture{}}, {&snapshotFixture{}, assets}} {
		if _, err := NewExecutor(ports.s, ports.a); err == nil {
			t.Fatal("nil port accepted")
		}
	}
}
