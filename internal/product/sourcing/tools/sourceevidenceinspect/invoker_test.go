package sourceevidenceinspect

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
	"task-processor/internal/integration/commercetoolauth"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

type auditRecorder struct {
	records []commercetool.AuditRecord
	fail    bool
}

func (r *auditRecorder) RecordToolCall(_ context.Context, record commercetool.AuditRecord) error {
	r.records = append(r.records, record)
	if r.fail {
		return errors.New("private-audit-backend")
	}
	return nil
}

func invocationFixture(t *testing.T) (context.Context, *snapshotStub, *sourceStub, commercetool.AgentDefinition, commercetool.InvocationDependencies, *auditRecorder) {
	t.Helper()
	ctx, _, c, s := bindingFixture()
	s.value = projectionFixture()
	c.value.Snapshot = s.value.Snapshot
	policy, err := authz.NewListingKitAuthorizer(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	permission, err := commercetoolauth.NewCasbinAuthorizer(policy)
	if err != nil {
		t.Fatal(err)
	}
	audit := &auditRecorder{}
	deps := commercetool.InvocationDependencies{PrincipalResolver: commercetoolauth.ContextPrincipalResolver{}, Authorizer: permission, Recorder: audit, Tracer: otel.Tracer("source-evidence-test"), Now: time.Now, AuditTimeout: time.Second}
	agent := commercetool.AgentDefinition{ID: "source.evidence.test", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{Definition().Ref, {ID: "product.other.inspect", Version: "v1.0.0"}}}
	return ctx, c, s, agent, deps, audit
}
func callMetadata() commercetool.CallMetadata {
	return commercetool.CallMetadata{CallID: "call-1", AgentID: "source.evidence.test", AgentVersion: "v1.0.0", AgentRunID: "synthetic-run", BusinessTaskID: "synthetic-correlation-only"}
}

func TestInvokerTraversesRegistryAndAudit(t *testing.T) {
	ctx, c, s, agent, deps, audit := invocationFixture(t)
	i, err := NewInvoker(c, s, agent, deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := i.Invoke(ctx, callMetadata(), Input{ProductKey: "product", CatalogVersion: "9007199254740993"})
	if err != nil || len(result.Output) == 0 || result.AuditStatus != commercetool.AuditStatusRecorded || len(audit.records) != 1 || c.calls != 1 || s.calls != 1 {
		t.Fatalf("invocation: %s %v audit=%#v", result.Output, err, audit.records)
	}
	r := audit.records[0]
	if r.ToolID != Definition().Ref.ID || r.ToolVersion != "v1.0.0" || r.TenantID != "org" || r.UserID != "actor" || r.Permission != authz.PermissionProductSourcingWrite || r.Outcome != commercetool.AuditOutcomeSucceeded || r.InputHash == "" || r.OutputHash == "" || r.AIInvocationID != "" {
		t.Fatalf("audit=%#v", r)
	}
	audit.fail = true
	result, err = i.Invoke(ctx, callMetadata(), Input{ProductKey: "product", CatalogVersion: "9007199254740993"})
	if err != nil || result.AuditStatus != commercetool.AuditStatusRecordFailed {
		t.Fatalf("audit failure hidden: %#v %v", result, err)
	}
}

func TestInvokerPreservesToolAllowlistAndCurrentPermission(t *testing.T) {
	ctx, c, s, agent, deps, _ := invocationFixture(t)
	for _, refs := range [][]commercetool.ToolRef{nil, {{ID: "other", Version: "v1.0.0"}}, {Definition().Ref, Definition().Ref}} {
		candidate := agent
		candidate.AllowedTools = refs
		if _, err := NewInvoker(c, s, candidate, deps); err == nil {
			t.Fatal("invalid allowlist accepted")
		}
	}
	i, err := NewInvoker(c, s, agent, deps)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := authidentity.AuthenticatedIdentityFromContext(ctx)
	id.Roles = []string{"listingkit_viewer"}
	ctx = authidentity.WithAuthenticatedIdentity(ctx, id)
	result, err := i.Invoke(ctx, callMetadata(), Input{ProductKey: "product", CatalogVersion: "9007199254740993"})
	if commercetool.CodeOf(err) != commercetool.ErrorPermissionDenied || len(result.Output) != 0 || c.calls != 0 || s.calls != 0 {
		t.Fatalf("permission bypass: %s %v", result.Output, err)
	}
	var empty *Invoker
	if _, err := empty.Invoke(ctx, callMetadata(), Input{}); err == nil {
		t.Fatal("nil invoker accepted")
	}
}

func TestRegistryRejectsUntrustedArgumentsBeforeOwnerReads(t *testing.T) {
	ctx, c, s, agent, deps, _ := invocationFixture(t)
	e, _ := NewExecutor(c, s)
	registry, err := commercetool.NewRegistry(commercetool.Tool{Definition: Definition(), Executor: e})
	if err != nil {
		t.Fatal(err)
	}
	agent.AllowedTools = []commercetool.ToolRef{Definition().Ref}
	bound, err := registry.Bind(agent, deps)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{"task_id":"legacy"}`, `{"product_key":"product","catalog_version":"1","tenant_id":"other"}`, `{"product_key":"product","catalog_version":1}`,
		`{"product_key":"product","catalog_version":"0"}`, `{"product_key":"product","catalog_version":"01"}`, `{"product_key":"product","catalog_version":"9223372036854775808"}`,
		`{"product_key":"product","catalog_version":"1","publication_id":"other"}`, `{"product_key":"product","catalog_version":"1","roles":["admin"]}`,
		`{"product_key":"` + strings.Repeat("a", 129) + `","catalog_version":"1"}`,
	} {
		result, err := bound.Invoke(ctx, commercetool.Call{Tool: Definition().Ref, Metadata: callMetadata(), Arguments: json.RawMessage(raw)})
		if commercetool.CodeOf(err) != commercetool.ErrorInvalidInput || len(result.Output) != 0 || c.calls != 0 || s.calls != 0 {
			t.Fatalf("raw input %s: %v", raw, err)
		}
	}
	result, err := bound.Invoke(ctx, commercetool.Call{Tool: Definition().Ref, Metadata: callMetadata(), Arguments: json.RawMessage(strings.Repeat("x", commercetool.MaxInvocationArgumentsBytes+1))})
	if commercetool.CodeOf(err) != commercetool.ErrorInvalidInput || len(result.Output) != 0 || c.calls != 0 {
		t.Fatal("oversized arguments accepted")
	}
}

func TestInvokerSafeErrorTaxonomyAndNoPartialOutput(t *testing.T) {
	cases := []struct {
		cause   error
		code    commercetool.ErrorCode
		catalog bool
	}{
		{sourcing.ErrPublicationForbidden, commercetool.ErrorPermissionDenied, false}, {sourcing.ErrSourcePublicationNotFound, commercetool.ErrorNotFound, false},
		{sourcing.ErrSourcePublicationStateInvalid, commercetool.ErrorInternal, false}, {sourcing.ErrSourcePublicationUnavailable, commercetool.ErrorDependencyUnavailable, false},
		{catalog.ErrSnapshotNotReady, commercetool.ErrorNotFound, true}, {catalog.ErrSnapshotTooLarge, commercetool.ErrorFailedPrecondition, true},
		{catalog.ErrRepositoryUnavailable, commercetool.ErrorDependencyUnavailable, true}, {context.Canceled, commercetool.ErrorDeadlineExceeded, false},
		{errors.New("private-database-secret"), commercetool.ErrorInternal, false},
	}
	for _, tc := range cases {
		ctx, c, s, agent, deps, audit := invocationFixture(t)
		if tc.catalog {
			c.err = tc.cause
		} else {
			s.err = tc.cause
		}
		i, _ := NewInvoker(c, s, agent, deps)
		result, err := i.Invoke(ctx, callMetadata(), Input{ProductKey: "product", CatalogVersion: "9007199254740993"})
		if commercetool.CodeOf(err) != tc.code || len(result.Output) != 0 || strings.Contains(err.Error(), "private") || len(audit.records) != 1 || audit.records[0].Outcome != commercetool.AuditOutcomeFailed {
			t.Fatalf("cause=%v result=%s err=%v audit=%#v", tc.cause, result.Output, err, audit.records)
		}
	}
}
