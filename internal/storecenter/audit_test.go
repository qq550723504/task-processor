package storecenter_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/storecenter"
)

func TestAuditRepositoryReplaysOnlyExactSafeEventAndScopesReads(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.TempDir()+"/audit.db?_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("SQLite DB handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := storecenter.AutoMigrateAuditRepository(nil); err == nil {
		t.Fatal("AutoMigrateAuditRepository(nil) error = nil")
	}
	if err := storecenter.AutoMigrateAuditRepository(db); err != nil {
		t.Fatalf("migrate audit: %v", err)
	}
	if err := storecenter.AutoMigrateAuditRepository(db); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	repository, err := storecenter.NewGormAuditRepository(db)
	if err != nil {
		t.Fatalf("NewGormAuditRepository: %v", err)
	}
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	event := safeAuditEvent("org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), now)

	stored, replayed, err := repository.Record(context.Background(), event)
	if err != nil || replayed {
		t.Fatalf("first Record() = (%+v, %v, %v), want durable non-replay", stored, replayed, err)
	}
	replay := event
	replay.EventID = uuid.NewString()
	replay.ActorSubject = "actor-other"
	durable, replayed, err := repository.Record(context.Background(), replay)
	if err != nil || !replayed || durable.ActorSubject != event.ActorSubject {
		t.Fatalf("cross-actor Record() = (%+v, %v, %v), want first durable actor replay", durable, replayed, err)
	}

	exact := event
	exact.EventID = uuid.NewString()
	exact.OccurredAt = now.Add(time.Hour)
	durable, replayed, err = repository.Record(context.Background(), exact)
	if err != nil || !replayed {
		t.Fatalf("exact Record() = (%+v, %v, %v), want original replay", durable, replayed, err)
	}
	if durable.EventID != event.EventID || !durable.OccurredAt.Equal(now) {
		t.Fatalf("replay durable authority = %+v, want first event ID/time", durable)
	}
	if _, err := repository.Get(context.Background(), "org-b", event.RequestKey, event.Action); !errors.Is(err, storecenter.ErrNotFound) {
		t.Fatalf("cross-org Get() error = %v, want ErrNotFound", err)
	}

	got, err := repository.Get(context.Background(), "org-a", event.RequestKey, event.Action)
	if err != nil {
		t.Fatalf("same-org Get(): %v", err)
	}
	if !reflect.DeepEqual(got.SafeFieldNames, []string{"lifecycle_status", "name"}) {
		t.Fatalf("safe fields = %v, want sorted/deduplicated allowlist", got.SafeFieldNames)
	}
}

func TestAuditMigrationCreatesCreatedAtScopedIndex(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	if err := storecenter.AutoMigrateAuditRepository(db); err != nil {
		t.Fatalf("migrate audit: %v", err)
	}
	var columns []struct{ Name string }
	if err := db.Raw("PRAGMA table_info(workbench_store_audit_logs)").Scan(&columns).Error; err != nil {
		t.Fatalf("table info: %v", err)
	}
	var indexes []struct{ Name string }
	if err := db.Raw("PRAGMA index_list(workbench_store_audit_logs)").Scan(&indexes).Error; err != nil {
		t.Fatalf("index list: %v", err)
	}
	hasColumn, hasIndex := false, false
	for _, column := range columns {
		hasColumn = hasColumn || column.Name == "created_at"
	}
	for _, index := range indexes {
		hasIndex = hasIndex || index.Name == "idx_workbench_store_audit_org_store_created"
	}
	if !hasColumn || !hasIndex {
		t.Fatalf("audit schema created_at/index = %v/%v, want both", hasColumn, hasIndex)
	}
	var indexColumns []struct{ Name string }
	if err := db.Raw("PRAGMA index_info(idx_workbench_store_audit_org_store_created)").Scan(&indexColumns).Error; err != nil {
		t.Fatalf("audit index info: %v", err)
	}
	got := make([]string, 0, len(indexColumns))
	for _, column := range indexColumns {
		got = append(got, column.Name)
	}
	if !reflect.DeepEqual(got, []string{"organization_id", "store_id", "created_at"}) {
		t.Fatalf("audit index columns = %v, want organization/store/created", got)
	}
}

func TestAuditRepositoryConcurrentExactRecordConverges(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.TempDir()+"/concurrent-audit.db?_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("DB handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := storecenter.AutoMigrateAuditRepository(db); err != nil {
		t.Fatalf("migrate audit: %v", err)
	}
	repository, err := storecenter.NewGormAuditRepository(db)
	if err != nil {
		t.Fatalf("NewGormAuditRepository: %v", err)
	}
	event := safeAuditEvent("org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), time.Now().UTC())
	start := make(chan struct{})
	errorsOut := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			candidate := event
			candidate.EventID = uuid.NewString()
			_, _, err := repository.Record(context.Background(), candidate)
			errorsOut <- err
		}()
	}
	close(start)
	for range 2 {
		if err := <-errorsOut; err != nil {
			t.Fatalf("concurrent Record() error = %v", err)
		}
	}
}

func TestAuditEventRejectsCredentialShapedFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	repository, err := storecenter.NewGormAuditRepository(db)
	if err != nil {
		t.Fatalf("NewGormAuditRepository: %v", err)
	}
	event := safeAuditEvent("org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), time.Now().UTC())
	event.SafeFieldNames = []string{"token"}
	if _, _, err := repository.Record(context.Background(), event); err == nil {
		t.Fatal("Record() with token-shaped safe field succeeded")
	}
	for _, typ := range []reflect.Type{reflect.TypeOf(storecenter.AuditEvent{}), reflect.TypeOf(storecenter.CreateStoreRequest{}), reflect.TypeOf(storecenter.CreateStoreResult{})} {
		for index := 0; index < typ.NumField(); index++ {
			field := typ.Field(index)
			if field.PkgPath != "" || strings.Contains(field.Tag.Get("json"), "-") {
				continue
			}
			name := strings.ToLower(field.Name + " " + strings.Split(field.Tag.Get("json"), ",")[0])
			for _, forbidden := range []string{"password", "token", "cookie", "secret", "credential", "username"} {
				if strings.Contains(name, forbidden) {
					t.Fatalf("%s exposes forbidden credential-shaped field %q", typ.Name(), field.Name)
				}
			}
		}
	}
}

func safeAuditEvent(organizationID, storeID, allocationID, requestKey string, occurredAt time.Time) storecenter.AuditEvent {
	return storecenter.AuditEvent{EventID: uuid.NewString(), OrganizationID: organizationID, StoreID: storeID, AllocationID: allocationID, RequestKey: requestKey, Action: storecenter.AuditActionQuotaReserved, Outcome: storecenter.AuditOutcomeSucceeded, ActorSubject: "actor-1", SafeFieldNames: []string{"name", "lifecycle_status", "name"}, NewState: storecenter.StoreStatusProvisioning, OccurredAt: occurredAt}
}
