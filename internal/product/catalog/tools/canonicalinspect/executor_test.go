package canonicalinspect

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"task-processor/internal/commercetool"
	listingtask "task-processor/internal/listing/task"
	"task-processor/internal/product/catalog"
)

func TestExecutorRejectsInvalidConstruction(t *testing.T) {
	reader := &catalogReaderStub{}
	checker := tenantAdminCheckerStub{}
	if _, err := NewExecutor(nil, reader, checker); err == nil {
		t.Fatal("NewExecutor(nil, reader) error = nil")
	}
	if _, err := NewExecutor(&subjectReaderStub{}, nil, checker); err == nil {
		t.Fatal("NewExecutor(subject, nil) error = nil")
	}
	if _, err := NewExecutor(&subjectReaderStub{}, reader, nil); err == nil {
		t.Fatal("NewExecutor(subject, reader, nil) error = nil")
	}
}

func TestExecutorPinnedAndLegacyReadSelection(t *testing.T) {
	for _, version := range []uint64{0, 7} {
		t.Run(map[bool]string{true: "legacy current", false: "pinned version"}[version == 0], func(t *testing.T) {
			subjects := &subjectReaderStub{subject: validSubject(version)}
			catalogs := &catalogReaderStub{published: catalog.PublishedSnapshot{
				Identity: catalog.SnapshotIdentity{TenantID: "tenant-1", ProductKey: "product-1"},
				Version:  7, PublicationID: "publication-1", Snapshot: catalog.ProductSnapshot{Title: "Bottle"},
			}}
			executor, err := newTestExecutor(subjects, catalogs)
			if err != nil {
				t.Fatalf("NewExecutor(): %v", err)
			}

			result, err := invokeExecutor(t, executor, "task-1", "task-1", schemaPrincipal())
			if err != nil {
				t.Fatalf("Invoke(): %v", err)
			}
			if subjects.calls != 1 || subjects.actor.TenantID != "tenant-1" || subjects.actor.UserID != "user-1" {
				t.Fatalf("subject calls/actor = %d %#v", subjects.calls, subjects.actor)
			}
			if version == 0 && (catalogs.currentCalls != 1 || catalogs.versionedCalls != 0) {
				t.Fatalf("legacy calls current=%d versioned=%d", catalogs.currentCalls, catalogs.versionedCalls)
			}
			if version > 0 && (catalogs.currentCalls != 0 || catalogs.versionedCalls != 1 || catalogs.requestedVersion != version) {
				t.Fatalf("pinned calls current=%d versioned=%d requested=%d", catalogs.currentCalls, catalogs.versionedCalls, catalogs.requestedVersion)
			}
			var output Output
			if err := json.Unmarshal(result.Output, &output); err != nil {
				t.Fatalf("unmarshal output: %v", err)
			}
			if output.SnapshotVersion != 7 || output.Snapshot.Title != "Bottle" || result.AIInvocationID != "" {
				t.Fatalf("result = %#v output = %#v", result, output)
			}
		})
	}
}

func TestExecutorRejectsBusinessTaskMismatchBeforeReaders(t *testing.T) {
	subjects := &subjectReaderStub{subject: validSubject(1)}
	catalogs := &catalogReaderStub{}
	executor, _ := newTestExecutor(subjects, catalogs)

	_, err := invokeExecutor(t, executor, "task-input", "task-metadata", schemaPrincipal())

	if commercetool.CodeOf(err) != commercetool.ErrorInvalidInput || subjects.calls != 0 || catalogs.currentCalls != 0 || catalogs.versionedCalls != 0 {
		t.Fatalf("error=%v subjectCalls=%d current=%d versioned=%d", err, subjects.calls, catalogs.currentCalls, catalogs.versionedCalls)
	}
}

func TestExecutorRechecksSubjectScope(t *testing.T) {
	tests := []listingtask.CanonicalSubject{
		{TaskID: "other", TenantID: "tenant-1", OwnerUserID: "user-1", ProductKey: "product-1", SnapshotVersion: 1},
		{TaskID: "task-1", TenantID: "tenant-2", OwnerUserID: "user-1", ProductKey: "product-1", SnapshotVersion: 1},
		{TaskID: "task-1", TenantID: "tenant-1", OwnerUserID: "other", ProductKey: "product-1", SnapshotVersion: 1},
	}
	for _, subject := range tests {
		subjects := &subjectReaderStub{subject: subject}
		catalogs := &catalogReaderStub{}
		executor, _ := newTestExecutor(subjects, catalogs)
		_, err := invokeExecutor(t, executor, "task-1", "task-1", schemaPrincipal())
		if commercetool.CodeOf(err) != commercetool.ErrorNotFound || catalogs.currentCalls+catalogs.versionedCalls != 0 {
			t.Fatalf("subject=%#v error=%v catalog calls=%d", subject, err, catalogs.currentCalls+catalogs.versionedCalls)
		}
	}
}

func TestExecutorRechecksSubjectScopeWithInjectedAdminSemantics(t *testing.T) {
	subjects := &subjectReaderStub{subject: canonicalSubjectForConfiguredAdmin()}
	catalogs := &catalogReaderStub{published: catalog.PublishedSnapshot{
		Identity: catalog.SnapshotIdentity{TenantID: "tenant-1", ProductKey: "product-1"}, Version: 1,
		Snapshot: catalog.ProductSnapshot{Title: "Bottle"},
	}}
	executor, err := NewExecutor(subjects, catalogs, tenantAdminCheckerStub{allow: true})
	if err != nil {
		t.Fatalf("NewExecutor(): %v", err)
	}
	if _, err := invokeExecutor(t, executor, "task-1", "task-1", schemaPrincipal()); err != nil {
		t.Fatalf("Invoke(): %v", err)
	}
}

func TestExecutorMapsStableDependencyErrors(t *testing.T) {
	tests := []struct {
		name       string
		subjectErr error
		catalogErr error
		want       commercetool.ErrorCode
	}{
		{name: "subject not found", subjectErr: listingtask.ErrCanonicalSubjectNotFound, want: commercetool.ErrorNotFound},
		{name: "subject not ready", subjectErr: listingtask.ErrCanonicalSubjectNotReady, want: commercetool.ErrorFailedPrecondition},
		{name: "invalid actor", subjectErr: listingtask.ErrInvalidActor, want: commercetool.ErrorIdentityIntegrity},
		{name: "subject repository unavailable", subjectErr: listingtask.ErrCanonicalSubjectUnavailable, want: commercetool.ErrorDependencyUnavailable},
		{name: "snapshot not ready", catalogErr: catalog.ErrSnapshotNotReady, want: commercetool.ErrorFailedPrecondition},
		{name: "repository unavailable", catalogErr: catalog.ErrRepositoryUnavailable, want: commercetool.ErrorDependencyUnavailable},
		{name: "repository state invalid", catalogErr: catalog.ErrRepositoryStateInvalid, want: commercetool.ErrorInternal},
		{name: "unknown", catalogErr: errors.New("database secret"), want: commercetool.ErrorInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subjects := &subjectReaderStub{subject: validSubject(1), err: tt.subjectErr}
			catalogs := &catalogReaderStub{err: tt.catalogErr}
			executor, _ := newTestExecutor(subjects, catalogs)
			_, err := invokeExecutor(t, executor, "task-1", "task-1", schemaPrincipal())
			if commercetool.CodeOf(err) != tt.want {
				t.Fatalf("CodeOf(error) = %s, want %s; error=%v", commercetool.CodeOf(err), tt.want, err)
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatalf("error leaked dependency detail: %v", err)
			}
		})
	}
}

func TestExecutorMapsOversizedProjectionToFailedPrecondition(t *testing.T) {
	subjects := &subjectReaderStub{subject: validSubject(1)}
	catalogs := &catalogReaderStub{published: catalog.PublishedSnapshot{
		Identity: catalog.SnapshotIdentity{TenantID: "tenant-1", ProductKey: "product-1"}, Version: 1,
		Snapshot: catalog.ProductSnapshot{Description: strings.Repeat("x", MaxOutputBytes)},
	}}
	executor, _ := newTestExecutor(subjects, catalogs)
	_, err := invokeExecutor(t, executor, "task-1", "task-1", schemaPrincipal())
	if commercetool.CodeOf(err) != commercetool.ErrorFailedPrecondition {
		t.Fatalf("CodeOf(error) = %s, error=%v", commercetool.CodeOf(err), err)
	}
}

type subjectReaderStub struct {
	subject listingtask.CanonicalSubject
	err     error
	calls   int
	actor   listingtask.Actor
}

func (s *subjectReaderStub) ReadCanonicalSubject(_ context.Context, actor listingtask.Actor, _ string) (listingtask.CanonicalSubject, error) {
	s.calls++
	s.actor = actor
	return s.subject.Clone(), s.err
}

type catalogReaderStub struct {
	published        catalog.PublishedSnapshot
	err              error
	currentCalls     int
	versionedCalls   int
	requestedVersion uint64
}

func (s *catalogReaderStub) GetCurrentSnapshot(context.Context, catalog.SnapshotIdentity) (catalog.PublishedSnapshot, error) {
	s.currentCalls++
	return s.published, s.err
}

func (s *catalogReaderStub) GetSnapshot(_ context.Context, _ catalog.SnapshotIdentity, version uint64) (catalog.PublishedSnapshot, error) {
	s.versionedCalls++
	s.requestedVersion = version
	return s.published, s.err
}

func validSubject(version uint64) listingtask.CanonicalSubject {
	return listingtask.CanonicalSubject{TaskID: "task-1", TenantID: "tenant-1", OwnerUserID: "user-1", ProductKey: "product-1", SnapshotVersion: version, Source: &listingtask.SourceLineage{Key: "source-1"}}
}

func canonicalSubjectForConfiguredAdmin() listingtask.CanonicalSubject {
	return listingtask.CanonicalSubject{TaskID: "task-1", TenantID: "tenant-1", OwnerUserID: "other-user", ProductKey: "product-1", SnapshotVersion: 1}
}

type tenantAdminCheckerStub struct{ allow bool }

func (s tenantAdminCheckerStub) IsTenantAdmin(string, []string) bool { return s.allow }

func newTestExecutor(subjects listingtask.CanonicalSubjectReader, catalogs snapshotReader) (*Executor, error) {
	return NewExecutor(subjects, catalogs, tenantAdminCheckerStub{})
}

func schemaPrincipal() commercetool.Principal {
	return commercetool.Principal{TenantID: "tenant-1", UserID: "user-1", Roles: []string{"listingkit_operator"}}
}

func invokeExecutor(t *testing.T, executor commercetool.Executor, inputTaskID, businessTaskID string, principal commercetool.Principal) (commercetool.Result, error) {
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
	arguments, _ := json.Marshal(Input{TaskID: inputTaskID})
	return bound.Invoke(context.Background(), commercetool.Call{
		Tool:      definition.Ref,
		Metadata:  commercetool.CallMetadata{CallID: "call-1", AgentID: "test.agent", AgentVersion: "v1.0.0", AgentRunID: "run-1", BusinessTaskID: businessTaskID},
		Arguments: arguments,
	})
}

type fixedPrincipalResolver struct{ principal commercetool.Principal }

func (r fixedPrincipalResolver) ResolvePrincipal(context.Context) (commercetool.Principal, error) {
	return r.principal, nil
}
