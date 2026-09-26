//go:build integration

package subjectverification

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	domain "task-processor/internal/subjectverification"
)

func TestPostgresReservationAndAtomicObservation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("verification"), tcpostgres.WithUsername("verification"), tcpostgres.WithPassword("verification"), tcpostgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = InstallSchemaTx(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	r, err := NewRepository(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	a := domain.Application{ID: "app-1", Scope: "scope", OrganizationID: "org-1", ActorID: "actor", IdempotencyKey: "key", InputDigest: "digest", CompanyName: "公司", CreditCode: "91310000MA00000001", PhoneDigest: "phone", MaskedPhone: "138****0001", Correlation: "correlation", State: domain.Unknown, CreatedAt: time.Now()}
	var created atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, newRow, e := r.Reserve(ctx, a)
			if e != nil {
				t.Error(e)
			}
			if newRow {
				created.Add(1)
			}
		}()
	}
	wg.Wait()
	if created.Load() != 1 {
		t.Fatalf("created %d applications", created.Load())
	}
	receipt := domain.Receipt{Scope: "scope", MessageID: "msg", Digest: "digest", Correlation: a.Correlation}
	// A database failure after the domain transition must roll back both facts.
	if err = db.Exec(`CREATE FUNCTION public.reject_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected receipt failure'; END $$; CREATE TRIGGER reject_receipt BEFORE UPDATE ON public.subject_verification_messages FOR EACH ROW EXECUTE FUNCTION public.reject_receipt()`).Error; err != nil {
		t.Fatal(err)
	}
	apply := func(a *domain.Application) string { a.State = domain.Verified; a.EncryptedURL = nil; return "VERIFIED" }
	if err = r.Apply(ctx, receipt, apply); err == nil {
		t.Fatal("expected transaction failure")
	}
	got, _ := r.Read(ctx, "org-1")
	if got.State != domain.Unknown {
		t.Fatal("partial state committed")
	}
	if err = db.Exec(`DROP TRIGGER reject_receipt ON public.subject_verification_messages`).Error; err != nil {
		t.Fatal(err)
	}
	if err = r.Apply(ctx, receipt, apply); err != nil {
		t.Fatal(err)
	}
	if err = r.SaveLink(ctx, a.ID, []byte("late ciphertext"), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	r, _ = NewRepository(ctx, db)
	if err = r.Apply(ctx, receipt, func(*domain.Application) string { t.Error("replayed callback"); return "" }); err != nil {
		t.Fatal(err)
	}
	receipt.Digest = "different"
	if err = r.Apply(ctx, receipt, apply); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("conflicting message: %v", err)
	}
	changedCorrelation := receipt
	changedCorrelation.Correlation = "unrelated-correlation"
	if err = r.Apply(ctx, changedCorrelation, apply); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("replayed message with unknown correlation must conflict: %v", err)
	}
	changedCorrelation.Digest = "digest"
	if err = r.Apply(ctx, changedCorrelation, apply); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("receipt cannot be reassociated even with the same digest: %v", err)
	}
	changedCorrelation.Correlation = ""
	if err = r.Apply(ctx, changedCorrelation, apply); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("receipt cannot discard its correlation: %v", err)
	}
	changedCorrelation.MessageID = "unrelated-message"
	if err = r.Apply(ctx, changedCorrelation, apply); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("new unrelated callback: %v", err)
	}
	got, _ = r.Read(ctx, "org-1")
	if got.State != domain.Verified || len(got.EncryptedURL) != 0 {
		t.Fatalf("late link overwrote verification: %s", got.State)
	}
	if _, err = r.Read(ctx, "org-2"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("cross-organization read")
	}
	if err = db.Exec(`ALTER TABLE public.subject_verification_applications DROP CONSTRAINT subject_verification_applications_organization_id_key`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = NewRepository(ctx, db); err == nil {
		t.Fatal("runtime admitted missing single-application constraint")
	}
}
