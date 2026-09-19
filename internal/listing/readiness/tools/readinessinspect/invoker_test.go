package readinessinspect

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"task-processor/internal/commercetool"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

type exactProducts struct {
	product catalog.PublishedSnapshot
	err     error
	calls   int
}

func (p *exactProducts) GetSnapshot(_ context.Context, id catalog.SnapshotIdentity, version uint64) (catalog.PublishedSnapshot, error) {
	p.calls++
	if p.err != nil {
		return catalog.PublishedSnapshot{}, p.err
	}
	if id != p.product.Identity || version != p.product.Version {
		return catalog.PublishedSnapshot{}, catalog.ErrSnapshotNotReady
	}
	return p.product, nil
}

type exactAssets struct {
	inventory *asset.ApprovedAssetInventory
	err       error
	calls     int
}

func (a *exactAssets) GetApprovedInventory(_ context.Context, scope asset.InventoryScope) (asset.ApprovedAssetInventory, error) {
	a.calls++
	if a.err != nil {
		return asset.ApprovedAssetInventory{}, a.err
	}
	if a.inventory == nil || a.inventory.Scope != scope {
		return asset.ApprovedAssetInventory{}, asset.ErrApprovedAssetsNotReady
	}
	return *a.inventory, nil
}

type freshPrincipal struct {
	calls   int
	revoked bool
	org     string
}

func (f *freshPrincipal) ResolveFreshPrincipal(context.Context) (commercetool.Principal, error) {
	f.calls++
	if f.revoked {
		return commercetool.Principal{}, errors.New("revoked")
	}
	return commercetool.Principal{TenantID: f.org, UserID: "user-1", Roles: []string{"listingkit_operator"}}, nil
}

type allowRead struct{}

func (allowRead) Authorize(context.Context, commercetool.Principal, commercetool.PermissionRequirement) error {
	return nil
}

type auditSink struct{}

func (auditSink) RecordToolCall(context.Context, commercetool.AuditRecord) error { return nil }

func testInvoker(t *testing.T, p *exactProducts, a *exactAssets, f *freshPrincipal) *Invoker {
	t.Helper()
	i, err := NewInvoker(p, a, f, commercetool.AgentDefinition{ID: "test.agent", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{Definition().Ref}}, commercetool.InvocationDependencies{
		Authorizer: allowRead{}, Recorder: auditSink{}, Tracer: otel.Tracer("readiness-test"), Now: time.Now, AuditTimeout: time.Second,
	})
	require.NoError(t, err)
	return i
}
func invoke(t *testing.T, i *Invoker, input Input) (commercetool.Result, error) {
	t.Helper()
	return i.Invoke(context.Background(), commercetool.CallMetadata{CallID: "call-1", AgentID: "test.agent", AgentVersion: "v1.0.0", AgentRunID: "run-1", BusinessTaskID: "correlation"}, input)
}
func validInput() Input {
	return Input{ProductKey: "product-1", CatalogVersion: "7", TargetPlatform: "shein"}
}

func TestInvokerExactReadAndFreshResolutionOnEveryCall(t *testing.T) {
	p, a, f := &exactProducts{product: productFixture()}, &exactAssets{inventory: inventoryFixture()}, &freshPrincipal{org: "org-a"}
	i := testInvoker(t, p, a, f)
	result, err := invoke(t, i, validInput())
	require.NoError(t, err)
	var out Output
	require.NoError(t, json.Unmarshal(result.Output, &out))
	require.Equal(t, "ready", out.Status)
	f.revoked = true
	_, err = invoke(t, i, validInput())
	require.Equal(t, commercetool.ErrorIdentityIntegrity, commercetool.CodeOf(err))
	require.Equal(t, 2, f.calls)
	require.Equal(t, 1, p.calls)
	require.Equal(t, 1, a.calls)
}

func TestInvokerFailsClosedForExactAndCorruptInputs(t *testing.T) {
	for _, tc := range []struct {
		name       string
		pErr, aErr error
		org        string
		version    string
		want       commercetool.ErrorCode
	}{
		{"wrong org", nil, nil, "other", "7", commercetool.ErrorNotFound},
		{"missing version", nil, nil, "org-a", "8", commercetool.ErrorNotFound},
		{"snapshot corruption", catalog.ErrRepositoryStateInvalid, nil, "org-a", "7", commercetool.ErrorInternal},
		{"asset failure", nil, errors.New("secret"), "org-a", "7", commercetool.ErrorInternal},
		{"snapshot bound", catalog.ErrSnapshotTooLarge, nil, "org-a", "7", commercetool.ErrorFailedPrecondition},
		{"cancel", context.Canceled, nil, "org-a", "7", commercetool.ErrorDeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, a, f := &exactProducts{product: productFixture(), err: tc.pErr}, &exactAssets{inventory: inventoryFixture(), err: tc.aErr}, &freshPrincipal{org: tc.org}
			i := testInvoker(t, p, a, f)
			input := validInput()
			input.CatalogVersion = tc.version
			result, err := invoke(t, i, input)
			require.Equal(t, tc.want, commercetool.CodeOf(err))
			require.Empty(t, result.Output)
			require.NotContains(t, err.Error(), "secret")
		})
	}
}

func TestInvokerMissingAssetIsDiagnosticAndTargetIsRequired(t *testing.T) {
	p, a, f := &exactProducts{product: productFixture()}, &exactAssets{}, &freshPrincipal{org: "org-a"}
	i := testInvoker(t, p, a, f)
	result, err := invoke(t, i, validInput())
	require.NoError(t, err)
	var out Output
	require.NoError(t, json.Unmarshal(result.Output, &out))
	require.Equal(t, "blocked", out.Status)
	for _, version := range []string{"0", "01", "+7", "9223372036854775808"} {
		input := validInput()
		input.CatalogVersion = version
		_, err = invoke(t, i, input)
		require.Equal(t, commercetool.ErrorInvalidInput, commercetool.CodeOf(err))
	}
	input := validInput()
	input.TargetPlatform = ""
	_, err = invoke(t, i, input)
	require.Equal(t, commercetool.ErrorInvalidInput, commercetool.CodeOf(err))
}
