package canonicalinspect

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"task-processor/internal/commercetool"
	"task-processor/internal/product/catalog"
)

func TestExecutorRejectsInvalidConstruction(t *testing.T) {
	if _, err := NewExecutor(nil); err == nil {
		t.Fatal("NewExecutor(nil) error = nil")
	}
}

func TestExecutorReadsOnlyExactTenantQualifiedVersion(t *testing.T) {
	const version = uint64(9007199254740993)
	reader := &catalogReaderStub{published: catalog.PublishedSnapshot{
		Identity: catalog.SnapshotIdentity{TenantID: "org-1", ProductKey: "product-1"},
		Version:  version, PublicationID: "publication-1", Snapshot: catalog.ProductSnapshot{Title: "Bottle"},
	}}
	executor, _ := NewExecutor(reader)

	result, err := invokeExecutor(t, executor, Input{ProductKey: "product-1", CatalogVersion: strconv.FormatUint(version, 10)}, schemaPrincipal())
	if err != nil {
		t.Fatalf("Invoke(): %v", err)
	}
	if reader.calls != 1 || reader.identity != (catalog.SnapshotIdentity{TenantID: "org-1", ProductKey: "product-1"}) || reader.version != version {
		t.Fatalf("reader calls=%d identity=%#v version=%d", reader.calls, reader.identity, reader.version)
	}
	var output Output
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if output.CatalogVersion != "9007199254740993" || output.ProductKey != "product-1" || output.CatalogPublicationID != "publication-1" || output.Snapshot.Title != "Bottle" {
		t.Fatalf("output = %#v", output)
	}
}

func TestParseCatalogVersionUsesPersistentInt64Domain(t *testing.T) {
	valid := strconv.FormatUint(math.MaxInt64, 10)
	if got, err := parseCatalogVersion(valid); err != nil || got != math.MaxInt64 {
		t.Fatalf("parse max = %d, %v", got, err)
	}
	for _, value := range []string{"", "0", "01", "+1", " 1", "9223372036854775808", "18446744073709551615"} {
		if _, err := parseCatalogVersion(value); err == nil {
			t.Fatalf("parseCatalogVersion(%q) error = nil", value)
		}
	}
}

func TestExecutorMapsStableCatalogErrorsWithoutDetails(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want commercetool.ErrorCode
	}{
		{name: "exact miss", err: catalog.ErrSnapshotNotReady, want: commercetool.ErrorNotFound},
		{name: "snapshot too large", err: catalog.ErrSnapshotTooLarge, want: commercetool.ErrorFailedPrecondition},
		{name: "repository unavailable", err: catalog.ErrRepositoryUnavailable, want: commercetool.ErrorDependencyUnavailable},
		{name: "repository state invalid", err: catalog.ErrRepositoryStateInvalid, want: commercetool.ErrorInternal},
		{name: "unknown", err: errors.New("database secret"), want: commercetool.ErrorInternal},
		{name: "canceled", err: context.Canceled, want: commercetool.ErrorDeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &catalogReaderStub{err: tt.err}
			executor, _ := NewExecutor(reader)
			_, err := invokeExecutor(t, executor, Input{ProductKey: "product-1", CatalogVersion: "1"}, schemaPrincipal())
			if commercetool.CodeOf(err) != tt.want {
				t.Fatalf("CodeOf(error) = %s, want %s; error=%v", commercetool.CodeOf(err), tt.want, err)
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatalf("error leaked dependency detail: %v", err)
			}
		})
	}
}

func TestExecutorRejectsMismatchedRepositoryState(t *testing.T) {
	for _, published := range []catalog.PublishedSnapshot{
		{Identity: catalog.SnapshotIdentity{TenantID: "other", ProductKey: "product-1"}, Version: 1, PublicationID: "publication-1"},
		{Identity: catalog.SnapshotIdentity{TenantID: "org-1", ProductKey: "other"}, Version: 1, PublicationID: "publication-1"},
		{Identity: catalog.SnapshotIdentity{TenantID: "org-1", ProductKey: "product-1"}, Version: 2, PublicationID: "publication-1"},
		{Identity: catalog.SnapshotIdentity{TenantID: "org-1", ProductKey: "product-1"}, Version: 1},
	} {
		reader := &catalogReaderStub{published: published}
		executor, _ := NewExecutor(reader)
		_, err := invokeExecutor(t, executor, Input{ProductKey: "product-1", CatalogVersion: "1"}, schemaPrincipal())
		if commercetool.CodeOf(err) != commercetool.ErrorInternal {
			t.Fatalf("published=%#v error=%v", published, err)
		}
	}
}

func TestExecutorMapsOversizedProjectionToFailedPrecondition(t *testing.T) {
	reader := &catalogReaderStub{published: validPublished(1)}
	reader.published.Snapshot.Description = strings.Repeat("x", MaxOutputBytes)
	executor, _ := NewExecutor(reader)
	_, err := invokeExecutor(t, executor, Input{ProductKey: "product-1", CatalogVersion: "1"}, schemaPrincipal())
	if commercetool.CodeOf(err) != commercetool.ErrorFailedPrecondition {
		t.Fatalf("CodeOf(error) = %s, error=%v", commercetool.CodeOf(err), err)
	}
}

func TestCanonicalInspectRejectsOversizeLegacyAndAuthorityArgumentsBeforeCatalog(t *testing.T) {
	reader := &catalogReaderStub{published: validPublished(1)}
	executor, _ := NewExecutor(reader)
	definition := Definition()
	registry, err := commercetool.NewRegistry(commercetool.Tool{Definition: definition, Executor: executor})
	if err != nil {
		t.Fatalf("NewRegistry(): %v", err)
	}
	bound, err := registry.Bind(commercetool.AgentDefinition{ID: "test.agent", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{definition.Ref}}, commercetool.InvocationDependencies{
		PrincipalResolver: fixedPrincipalResolver{principal: schemaPrincipal()}, Authorizer: schemaAuthorizer{}, Recorder: schemaAuditRecorder{},
		Tracer: otel.Tracer("canonicalinspect-rejection-test"), Now: time.Now, AuditTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("Bind(): %v", err)
	}
	for _, arguments := range []json.RawMessage{
		json.RawMessage(`{"task_id":"old-task"}`),
		json.RawMessage(`{"product_key":"product-1","catalog_version":1}`),
		json.RawMessage(`{"product_key":"product-1","catalog_version":"1","tenant_id":"org-1"}`),
		json.RawMessage(`{"product_key":"product-1","catalog_version":"1","roles":["admin"]}`),
		json.RawMessage(`{"product_key":"` + strings.Repeat("x", commercetool.MaxInvocationArgumentsBytes) + `","catalog_version":"1"}`),
	} {
		_, err := bound.Invoke(context.Background(), commercetool.Call{
			Tool:      definition.Ref,
			Metadata:  commercetool.CallMetadata{CallID: "rejected", AgentID: "test.agent", AgentVersion: "v1.0.0", AgentRunID: "run-1", BusinessTaskID: "correlation-only"},
			Arguments: arguments,
		})
		if commercetool.CodeOf(err) != commercetool.ErrorInvalidInput {
			t.Fatalf("arguments=%s code=%s error=%v", arguments[:min(len(arguments), 200)], commercetool.CodeOf(err), err)
		}
	}
	if reader.calls != 0 {
		t.Fatalf("catalog reader calls = %d, want 0", reader.calls)
	}
}

type catalogReaderStub struct {
	published catalog.PublishedSnapshot
	err       error
	calls     int
	identity  catalog.SnapshotIdentity
	version   uint64
}

func (s *catalogReaderStub) GetSnapshot(_ context.Context, identity catalog.SnapshotIdentity, version uint64) (catalog.PublishedSnapshot, error) {
	s.calls++
	s.identity = identity
	s.version = version
	return s.published, s.err
}

func validPublished(version uint64) catalog.PublishedSnapshot {
	return catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: "org-1", ProductKey: "product-1"}, Version: version, PublicationID: "publication-1", Snapshot: catalog.ProductSnapshot{Title: "Bottle"}}
}

func schemaPrincipal() commercetool.Principal {
	return commercetool.Principal{TenantID: "org-1", UserID: "user-1", Roles: []string{"listingkit_operator"}}
}

func invokeExecutor(t *testing.T, executor commercetool.Executor, input Input, principal commercetool.Principal) (commercetool.Result, error) {
	t.Helper()
	definition := Definition()
	registry, err := commercetool.NewRegistry(commercetool.Tool{Definition: definition, Executor: executor})
	if err != nil {
		t.Fatalf("NewRegistry(): %v", err)
	}
	bound, err := registry.Bind(commercetool.AgentDefinition{ID: "test.agent", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{definition.Ref}}, commercetool.InvocationDependencies{
		PrincipalResolver: fixedPrincipalResolver{principal: principal}, Authorizer: schemaAuthorizer{}, Recorder: schemaAuditRecorder{},
		Tracer: otel.Tracer("canonicalinspect-executor-test"), Now: time.Now, AuditTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("Bind(): %v", err)
	}
	arguments, _ := json.Marshal(input)
	return bound.Invoke(context.Background(), commercetool.Call{
		Tool:      definition.Ref,
		Metadata:  commercetool.CallMetadata{CallID: "call-1", AgentID: "test.agent", AgentVersion: "v1.0.0", AgentRunID: "run-1", BusinessTaskID: "correlation-only"},
		Arguments: arguments,
	})
}

type fixedPrincipalResolver struct{ principal commercetool.Principal }

func (r fixedPrincipalResolver) ResolvePrincipal(context.Context) (commercetool.Principal, error) {
	return r.principal, nil
}
