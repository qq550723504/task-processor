//go:build integration

package notificationcenterpersistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	n "task-processor/internal/notificationcenter"
)

func TestPostgresReceiptsAndRuntimeRole(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("notifications"), tcpostgres.WithUsername("test"), tcpostgres.WithPassword("test"), tcpostgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	open := func(dsn string) *gorm.DB {
		db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if e != nil {
			t.Fatal(e)
		}
		sql, e := db.DB()
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { _ = sql.Close() })
		return db
	}
	admin := open(dsn)
	if err = InstallSchema(admin); err != nil {
		t.Fatal(err)
	}
	if err = admin.Exec("CREATE ROLE notification_center_runtime LOGIN PASSWORD 'runtime-test'").Error; err != nil {
		t.Fatal(err)
	}
	if err = GrantRuntime(admin, "notification_center_runtime"); err != nil {
		t.Fatal(err)
	}
	runtime := open(strings.Replace(dsn, "test:test@", "notification_center_runtime:runtime-test@", 1))
	if err = VerifySchema(ctx, runtime); err != nil {
		t.Fatal("restricted role rejected", err)
	}
	for _, check := range []struct{ setup, undo string }{
		{"CREATE TABLE public.foreign_fact(id text); GRANT SELECT ON public.foreign_fact TO notification_center_runtime", "REVOKE SELECT ON public.foreign_fact FROM notification_center_runtime"},
		{"GRANT UPDATE(content) ON notification_center.announcements TO notification_center_runtime", "REVOKE UPDATE(content) ON notification_center.announcements FROM notification_center_runtime"},
		{"REVOKE DELETE ON notification_center.snapshots FROM notification_center_runtime", "GRANT DELETE ON notification_center.snapshots TO notification_center_runtime"},
		{"REVOKE UPDATE(state) ON notification_center.announcements FROM notification_center_runtime", "GRANT UPDATE(state) ON notification_center.announcements TO notification_center_runtime"},
	} {
		if err = admin.Exec(check.setup).Error; err != nil {
			t.Fatal(err)
		}
		if err = VerifySchema(ctx, runtime); err == nil {
			t.Fatalf("invalid role accepted: %s", check.setup)
		}
		if err = admin.Exec(check.undo).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo, _ := New(runtime)
	svc := n.Service{Repository: repo, Sources: []n.Source{Announcements{Store: repo}}}
	scope := n.Scope{Realm: "http://localhost:18080", Subject: "publisher", Category: n.Official}
	input := n.AnnouncementInput{Category: "PRODUCT", Title: "产品更新", Summary: "查看已发布公告", Paragraphs: []string{"正式内容"}, Target: n.Target{Kind: "home"}}
	published, err := svc.Publish(ctx, scope, uuid.NewString(), input)
	if err != nil {
		t.Fatal(err)
	}
	scope.Subject = "reader"
	list, err := svc.List(ctx, scope, n.ListRequest{Limit: 20})
	if err != nil || len(list.Items) != 1 {
		t.Fatal(list, err)
	}
	snapshot, err := svc.PrepareReadAll(ctx, scope, uuid.NewString())
	if err != nil {
		t.Fatal("snapshot TTL", err)
	}
	key := uuid.NewString()
	if _, err = svc.ReadAll(ctx, scope, key, snapshot.ID, snapshot.Fingerprint); err != nil {
		t.Fatal(err)
	}
	other := scope
	other.Subject = "other-reader"
	otherList, err := svc.List(ctx, other, n.ListRequest{Limit: 20})
	if err != nil || otherList.Unread != 1 {
		t.Fatal("receipt crossed identity", otherList, err)
	}
	if err = admin.Exec("DELETE FROM notification_center.snapshots WHERE id=?", snapshot.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = svc.ReadAll(ctx, scope, key, snapshot.ID, snapshot.Fingerprint); err != nil {
		t.Fatal("committed replay depended on expired snapshot", err)
	}
	if _, err = svc.ReadAll(ctx, scope, key, uuid.NewString(), snapshot.Fingerprint); !errors.Is(err, n.ErrConflict) {
		t.Fatal("different payload replay", err)
	}
	atomicScope := scope
	atomicScope.Subject = "atomic-reader"
	if err = admin.Exec("REVOKE INSERT ON notification_center.commands FROM notification_center_runtime").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Read(ctx, atomicScope, uuid.NewString(), list.Items[0].ID); !errors.Is(err, n.ErrUnavailable) {
		t.Fatal("failed command reported read success", err)
	}
	var receiptCount int64
	if err = admin.Table("notification_center.reading_receipts").Where("subject=?", atomicScope.Subject).Count(&receiptCount).Error; err != nil || receiptCount != 0 {
		t.Fatal("receipt escaped rollback", receiptCount, err)
	}
	if err = admin.Exec("GRANT INSERT ON notification_center.commands TO notification_center_runtime").Error; err != nil {
		t.Fatal(err)
	}
	limitScope := scope
	limitScope.Subject = "snapshot-limit-reader"
	for i := 0; i < 4; i++ {
		if _, err = svc.PrepareReadAll(ctx, limitScope, uuid.NewString()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = svc.PrepareReadAll(ctx, limitScope, uuid.NewString()); !errors.Is(err, n.ErrCapacity) {
		t.Fatal("active snapshot limit bypassed", err)
	}
	foreignRealm := scope
	foreignRealm.Realm = "http://other-realm.local"
	if _, err = svc.Detail(ctx, foreignRealm, list.Items[0].ID); !errors.Is(err, n.ErrNotFound) {
		t.Fatal("foreign realm detail leaked", err)
	}
	scope.Subject = "publisher"
	if _, err = svc.Withdraw(ctx, scope, uuid.NewString(), published.ResultID, 1); err != nil {
		t.Fatal(err)
	}
	scope.Subject = "reader"
	list, err = svc.List(ctx, scope, n.ListRequest{Limit: 20})
	if err != nil || list.Count != 0 {
		t.Fatal("withdrawn announcement visible", list, err)
	}
	scope.Subject = "publisher"
	long := input
	long.Paragraphs = []string{strings.Repeat("a", 30000)}
	for i := 0; i < 5; i++ {
		if _, err = svc.Publish(ctx, scope, uuid.NewString(), long); err != nil {
			t.Fatal(err)
		}
	}
	scope.Subject = "long-reader"
	list, err = svc.List(ctx, scope, n.ListRequest{Limit: 20})
	raw, _ := json.Marshal(list)
	if err != nil || list.Count != 5 || len(raw) > 128<<10 {
		t.Fatal("legal long announcements broke list", len(raw), list.Count, err)
	}
	detail, err := svc.Detail(ctx, scope, list.Items[0].ID)
	if err != nil || len(detail.Paragraphs) != 1 || len(detail.Paragraphs[0]) != 30000 {
		t.Fatal("long detail lost content", err)
	}
	if err = admin.Exec("REVOKE INSERT ON notification_center.commands FROM notification_center_runtime").Error; err != nil {
		t.Fatal(err)
	}
	if err = VerifySchema(ctx, runtime); err == nil {
		t.Fatal("SELECT-only command role accepted")
	}
}
