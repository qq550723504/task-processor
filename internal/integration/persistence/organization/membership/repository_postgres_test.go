//go:build integration

package membership

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	domain "task-processor/internal/organization/membership"
)

func TestPostgresReservationDispatchAndRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("membership_test"), tcpostgres.WithUsername("test_owner"), tcpostgres.WithPassword("task-only-password"), tcpostgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Error(err)
		}
	})
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := InstallSchemaTx(ctx, tx); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	original := domain.Operation{Scope: domain.OperationScope{ProjectID: "p", OrganizationID: "org", ActorID: "actor-a"}, Key: uuid.NewString(), Fingerprint: strings.Repeat("a", 64), Kind: domain.CommandRole, TargetUserID: "target", AuthorizationID: "grant", Role: "listingkit_viewer", ExpectedVersion: strings.Repeat("b", 64), Step: domain.StepRole, Phase: domain.PhaseReady, Revision: 1}
	other := original
	other.Scope.ActorID = "actor-b"
	other.Key = uuid.NewString()
	var wg sync.WaitGroup
	results := make(chan domain.Operation, 2)
	failures := make(chan error, 2)
	for _, candidate := range []domain.Operation{original, other} {
		wg.Add(1)
		go func(op domain.Operation) {
			defer wg.Done()
			got, err := repo.Begin(ctx, op)
			if err != nil {
				failures <- err
			} else {
				results <- got
			}
		}(candidate)
	}
	wg.Wait()
	close(results)
	close(failures)
	if len(results) != 1 || len(failures) != 1 {
		t.Fatalf("winners=%d failures=%d", len(results), len(failures))
	}
	if err := <-failures; !errors.Is(err, domain.ErrConflict) {
		t.Fatal(err)
	}
	winner := <-results
	same, err := repo.Begin(ctx, winner)
	if err != nil || same.Key != winner.Key {
		t.Fatalf("replay=%+v %v", same, err)
	}
	for _, field := range []string{"target_user_id='foreign'", "fingerprint='" + strings.Repeat("f", 64) + "'", "active=false", "invite_email='foreign@example.com'"} {
		tx, err := sqlDB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET `+field); err != nil {
			t.Fatal(err)
		}
		if _, err := read(ctx, tx, winner.Scope, winner.Key, false); err == nil {
			t.Errorf("receipt divergence admitted: %s", field)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
	}
	changed := winner
	changed.Fingerprint = strings.Repeat("c", 64)
	if _, err := repo.Begin(ctx, changed); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("different payload=%v", err)
	}
	claims := make(chan domain.Operation, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := repo.Apply(ctx, winner.Scope, winner.Key, 1, domain.OperationChange{Event: domain.EventDispatch, DispatchID: uuid.NewString()})
			if err == nil {
				claims <- got
			} else if !errors.Is(err, domain.ErrConflict) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	close(claims)
	if len(claims) != 1 {
		t.Fatalf("dispatch winners=%d", len(claims))
	}
	dispatched := <-claims
	rebuilt, err := NewRepository(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := rebuilt.Read(ctx, winner.Scope, winner.Key)
	if err != nil || persisted.Phase != domain.PhaseDispatched {
		t.Fatalf("restart=%+v %v", persisted, err)
	}
	if _, err := rebuilt.Apply(ctx, persisted.Scope, persisted.Key, persisted.Revision, domain.OperationChange{Event: domain.EventDispatch, DispatchID: uuid.NewString()}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("redispatch=%v", err)
	}
	alias := original
	alias.Key = uuid.NewString()
	alias.AuthorizationID = "new-visible-grant"
	if _, err := rebuilt.Begin(ctx, alias); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("alias bypass=%v", err)
	}
	wrongScope := winner.Scope
	wrongScope.OrganizationID = "foreign"
	if _, err := rebuilt.Read(ctx, wrongScope, winner.Key); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-org read=%v", err)
	}
	_, err = rebuilt.Apply(ctx, dispatched.Scope, dispatched.Key, dispatched.Revision, domain.OperationChange{Event: domain.EventAcknowledge, DispatchID: dispatched.DispatchID, Acknowledgment: &domain.Acknowledgment{ID: "grant", At: time.Now().UTC().Format(time.RFC3339Nano)}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rebuilt.Begin(ctx, alias); err != nil {
		t.Fatalf("valid ACK did not release: %v", err)
	}
	if err := VerifyRuntimePermissions(ctx, db); err == nil {
		t.Fatal("owner runtime admitted")
	}
	if err := db.Exec(`CREATE ROLE organization_membership_runtime LOGIN PASSWORD 'member-runtime-password';
 REVOKE CREATE,TEMPORARY ON DATABASE membership_test FROM PUBLIC;
 REVOKE CREATE ON SCHEMA public FROM PUBLIC;
 GRANT CONNECT ON DATABASE membership_test TO organization_membership_runtime;
 GRANT USAGE ON SCHEMA public TO organization_membership_runtime;
 GRANT SELECT,INSERT,UPDATE ON public.organization_member_operations TO organization_membership_runtime;
 CREATE TABLE public.unrelated_facts (id integer);`).Error; err != nil {
		t.Fatal(err)
	}
	connection, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	connection.User = url.UserPassword("organization_membership_runtime", "member-runtime-password")
	runtimeDB, err := gorm.Open(postgres.Open(connection.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	runtimeSQL, err := runtimeDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeSQL.Close()
	if err := VerifyRuntimePermissions(ctx, runtimeDB); err != nil {
		t.Fatalf("minimum runtime rejected: %v", err)
	}
	for _, extra := range []struct{ grant, revoke string }{
		{`GRANT SELECT(id) ON public.unrelated_facts TO organization_membership_runtime`, `REVOKE SELECT(id) ON public.unrelated_facts FROM organization_membership_runtime`},
		{`GRANT SELECT ON public.unrelated_facts TO PUBLIC`, `REVOKE SELECT ON public.unrelated_facts FROM PUBLIC`},
		{`CREATE ROLE membership_extra; GRANT DELETE ON public.organization_member_operations TO membership_extra; GRANT membership_extra TO organization_membership_runtime`, `REVOKE membership_extra FROM organization_membership_runtime`},
	} {
		if err := db.Exec(extra.grant).Error; err != nil {
			t.Fatal(err)
		}
		if err := VerifyRuntimePermissions(ctx, runtimeDB); err == nil {
			t.Fatal("extra effective privilege admitted")
		}
		if err := db.Exec(extra.revoke).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`REVOKE UPDATE ON public.organization_member_operations FROM organization_membership_runtime`).Error; err != nil {
		t.Fatal(err)
	}
	if err := VerifyRuntimePermissions(ctx, runtimeDB); err == nil {
		t.Fatal("missing update admitted")
	}
	for _, mutation := range []string{
		`ALTER TABLE public.organization_member_operations ALTER COLUMN target_user_id DROP NOT NULL`,
		`ALTER TABLE public.organization_member_operations ALTER COLUMN revision TYPE integer`,
		`ALTER TABLE public.organization_member_operations DROP CONSTRAINT organization_member_operations_revision_check`,
		`ALTER TABLE public.organization_member_operations DROP CONSTRAINT organization_member_operations_payload_check`,
	} {
		tx, err := sqlDB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.ExecContext(ctx, mutation); err != nil {
			t.Fatal(err)
		}
		// Catalog checks use the same connection while the task-only DDL is uncommitted.
		if err = verifySchema(ctx, tx); err == nil {
			t.Errorf("drift admitted: %s", mutation)
		}
		if err = tx.Rollback(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := sqlDB.ExecContext(ctx, `DROP INDEX public.organization_member_active_target`); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRepository(ctx, db); err == nil {
		t.Fatal("missing target reservation index admitted")
	}
}
