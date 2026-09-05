package tests

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
	"task-processor/internal/integration/commercetoolauth"
	catalogpersistence "task-processor/internal/integration/persistence/product/catalog"
	listingtask "task-processor/internal/listing/task"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
	listingstore "task-processor/internal/listingkit/store"
	"task-processor/internal/product/catalog"
	canonicalinspect "task-processor/internal/product/catalog/tools/canonicalinspect"
)

func TestRegistryConformanceCanonicalInspectionVerticalSlice(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: "file:" + t.Name() + "?mode=memory&cache=shared"}, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		t.Fatalf("migrate task: %v", err)
	}
	if err := catalogpersistence.AutoMigrate(db); err != nil {
		t.Fatalf("migrate catalog: %v", err)
	}
	casbin, err := authz.NewListingKitAuthorizer([]string{"configured-user"}, []string{"configured-role"})
	if err != nil {
		t.Fatalf("NewListingKitAuthorizer(): %v", err)
	}

	catalogRepository, err := catalogpersistence.NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository(): %v", err)
	}
	publisher, err := catalog.NewPublisher(catalogRepository)
	if err != nil {
		t.Fatalf("NewPublisher(): %v", err)
	}
	identity := catalog.SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-1"}
	first, err := publisher.Publish(context.Background(), catalog.PublishRequest{Identity: identity, PublicationID: "publication-1", Snapshot: catalog.ProductSnapshot{Title: "Bottle v1"}})
	if err != nil {
		t.Fatalf("publish v1: %v", err)
	}
	second, err := publisher.Publish(context.Background(), catalog.PublishRequest{Identity: identity, PublicationID: "publication-2", Snapshot: catalog.ProductSnapshot{Title: "Bottle v2"}})
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}

	tasks := []*listingkit.Task{
		conformanceTask("task-owner", "tenant-a", "owner-a", "product-1", first.Version),
		conformanceTask("task-other", "tenant-a", "owner-b", "product-1", first.Version),
		conformanceTask("task-legacy", "tenant-a", "owner-a", "product-1", 0),
		conformanceTask("task-not-ready", "tenant-a", "owner-a", "product-missing", 0),
		conformanceTask("task-foreign", "tenant-b", "owner-b", "product-1", first.Version),
	}
	for _, task := range tasks {
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("create task %s: %v", task.ID, err)
		}
	}

	taskRepository, err := listingstore.NewTaskRepositoryWithTenantAdminChecker(db, casbin)
	if err != nil {
		t.Fatalf("NewTaskRepositoryWithTenantAdminChecker(): %v", err)
	}
	subjects, ok := taskRepository.(listingtask.CanonicalSubjectReader)
	if !ok {
		t.Fatal("task repository does not implement CanonicalSubjectReader")
	}
	snapshots, err := catalogpersistence.NewBoundedSnapshotReader(db, canonicalinspect.MaxCatalogSnapshotBytes)
	if err != nil {
		t.Fatalf("NewBoundedSnapshotReader(): %v", err)
	}
	executor, err := canonicalinspect.NewExecutor(subjects, snapshots, casbin)
	if err != nil {
		t.Fatalf("NewExecutor(): %v", err)
	}
	authorizer, err := commercetoolauth.NewCasbinAuthorizer(casbin)
	if err != nil {
		t.Fatalf("NewCasbinAuthorizer(): %v", err)
	}

	spanRecorder := tracetest.NewSpanRecorder()
	traceProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))
	t.Cleanup(func() { _ = traceProvider.Shutdown(context.Background()) })
	audits := &conformanceAuditRecorder{}
	definition := canonicalinspect.Definition()
	registry, err := commercetool.NewRegistry(commercetool.Tool{Definition: definition, Executor: executor})
	if err != nil {
		t.Fatalf("NewRegistry(): %v", err)
	}
	bound, err := registry.Bind(commercetool.AgentDefinition{ID: "fake.product-agent", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{definition.Ref}}, commercetool.InvocationDependencies{
		PrincipalResolver: commercetoolauth.ContextPrincipalResolver{}, Authorizer: authorizer, Recorder: audits,
		Tracer: traceProvider.Tracer("canonicalinspect-conformance"), Now: time.Now, AuditTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("Bind(): %v", err)
	}

	before := captureConformanceState(t, db)

	owner := conformanceIdentity("tenant-a", "owner-a", "listingkit_operator")
	pinned := invokeConformance(t, bound, owner, "task-owner")
	assertConformanceTitle(t, pinned, "Bottle v1", first.Version)
	legacy := invokeConformance(t, bound, owner, "task-legacy")
	assertConformanceTitle(t, legacy, "Bottle v2", second.Version)

	assertConformanceError(t, bound, owner, "task-other", commercetool.ErrorNotFound)
	admin := conformanceIdentity("tenant-a", "admin-a", "listingkit_admin")
	assertConformanceTitle(t, invokeConformance(t, bound, admin, "task-other"), "Bottle v1", first.Version)
	platform := conformanceIdentity("tenant-a", "platform-a", "platform_admin")
	assertConformanceTitle(t, invokeConformance(t, bound, platform, "task-other"), "Bottle v1", first.Version)
	legacyPlatform := conformanceIdentity("tenant-a", "legacy-platform-a", "admin")
	assertConformanceTitle(t, invokeConformance(t, bound, legacyPlatform, "task-other"), "Bottle v1", first.Version)
	configuredRole := conformanceIdentity("tenant-a", "role-user", "configured-role")
	assertConformanceTitle(t, invokeConformance(t, bound, configuredRole, "task-other"), "Bottle v1", first.Version)
	configuredUser := conformanceIdentity("tenant-a", "configured-user", "listingkit_viewer")
	assertConformanceTitle(t, invokeConformance(t, bound, configuredUser, "task-other"), "Bottle v1", first.Version)
	assertConformanceError(t, bound, conformanceIdentity("tenant-b", "platform-a", "platform_admin"), "task-owner", commercetool.ErrorNotFound)
	assertConformanceError(t, bound, conformanceIdentity("tenant-b", "configured-user", "listingkit_viewer"), "task-owner", commercetool.ErrorNotFound)
	assertConformanceError(t, bound, conformanceIdentity("tenant-a", "viewer-a", "listingkit_viewer"), "task-owner", commercetool.ErrorPermissionDenied)
	assertConformanceError(t, bound, owner, "task-not-ready", commercetool.ErrorFailedPrecondition)

	after := captureConformanceState(t, db)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("read-only tool changed durable state\nbefore=%#v\nafter=%#v", before, after)
	}
	if len(audits.records) != 12 {
		t.Fatalf("audit records = %d, want 12", len(audits.records))
	}
	if len(spanRecorder.Ended()) != 11 {
		t.Fatalf("ended spans = %d, want 11 executor-bound calls", len(spanRecorder.Ended()))
	}
}

type conformanceAuditRecorder struct{ records []commercetool.AuditRecord }

func (r *conformanceAuditRecorder) RecordToolCall(_ context.Context, record commercetool.AuditRecord) error {
	r.records = append(r.records, record)
	return nil
}

func conformanceTask(taskID, tenantID, userID, productKey string, version uint64) *listingkit.Task {
	return &listingkit.Task{
		ID: taskID, TenantID: tenantID, UserID: userID, SourceSnapshotVersion: version, Status: core.TaskStatusPending,
		Request: &listingkit.GenerateRequest{TenantID: tenantID, UserID: userID, ProductKey: productKey, Source: &listingkit.SourceReference{Key: "source-" + productKey, Type: "crawler", Platform: "1688", ID: "123"}},
	}
}

func conformanceIdentity(tenantID, userID, role string) authidentity.AuthenticatedIdentity {
	return authidentity.AuthenticatedIdentity{TenantID: tenantID, UserID: userID, Roles: []string{role}}
}

func invokeConformance(t *testing.T, bound *commercetool.BoundToolSet, identity authidentity.AuthenticatedIdentity, taskID string) commercetool.Result {
	t.Helper()
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), identity)
	arguments, _ := json.Marshal(canonicalinspect.Input{TaskID: taskID})
	result, err := bound.Invoke(ctx, commercetool.Call{
		Tool:      canonicalinspect.Definition().Ref,
		Metadata:  commercetool.CallMetadata{CallID: "call-" + taskID + "-" + identity.UserID, AgentID: "fake.product-agent", AgentVersion: "v1.0.0", AgentRunID: "run-1", BusinessTaskID: taskID},
		Arguments: arguments,
	})
	if err != nil {
		t.Fatalf("Invoke(%s, %s) error = %v", identity.UserID, taskID, err)
	}
	return result
}

func assertConformanceError(t *testing.T, bound *commercetool.BoundToolSet, identity authidentity.AuthenticatedIdentity, taskID string, want commercetool.ErrorCode) {
	t.Helper()
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), identity)
	arguments, _ := json.Marshal(canonicalinspect.Input{TaskID: taskID})
	_, err := bound.Invoke(ctx, commercetool.Call{
		Tool:      canonicalinspect.Definition().Ref,
		Metadata:  commercetool.CallMetadata{CallID: "call-" + taskID + "-" + identity.UserID, AgentID: "fake.product-agent", AgentVersion: "v1.0.0", AgentRunID: "run-1", BusinessTaskID: taskID},
		Arguments: arguments,
	})
	if commercetool.CodeOf(err) != want {
		t.Fatalf("Invoke(%s, %s) code = %s, want %s; error=%v", identity.UserID, taskID, commercetool.CodeOf(err), want, err)
	}
}

func assertConformanceTitle(t *testing.T, result commercetool.Result, title string, version uint64) {
	t.Helper()
	var output canonicalinspect.Output
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if output.Snapshot.Title != title || output.SnapshotVersion != version || result.AIInvocationID != "" {
		t.Fatalf("output = %#v, result AI invocation = %q", output, result.AIInvocationID)
	}
}

type conformanceState struct {
	Tasks     []listingkit.Task
	TaskCount int64
	Versions  int64
	Heads     int64
}

func captureConformanceState(t *testing.T, db *gorm.DB) conformanceState {
	t.Helper()
	state := conformanceState{}
	if err := db.Order("id").Find(&state.Tasks).Error; err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if err := db.Model(&listingkit.Task{}).Count(&state.TaskCount).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if err := db.Table("product_snapshot_versions").Count(&state.Versions).Error; err != nil {
		t.Fatalf("count versions: %v", err)
	}
	if err := db.Table("product_snapshot_heads").Count(&state.Heads).Error; err != nil {
		t.Fatalf("count heads: %v", err)
	}
	return state
}
