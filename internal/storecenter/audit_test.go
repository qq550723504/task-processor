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
	replay.ActorSubject = "actor-other" // actor is part of the safe semantic identity; changing it must conflict.
	if _, _, err := repository.Record(context.Background(), replay); !errors.Is(err, storecenter.ErrAuditIdentityMismatch) {
		t.Fatalf("conflicting Record() error = %v, want ErrAuditIdentityMismatch", err)
	}

	exact := event
	exact.EventID = uuid.NewString()
	exact.OccurredAt = now.Add(time.Hour)
	durable, replayed, err := repository.Record(context.Background(), exact)
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
