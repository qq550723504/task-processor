//go:build integration

package referral

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	d "task-processor/internal/referral"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	pg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDatabaseLeaseAndCreatePermission(t *testing.T) {
	owner, runtime := ownedDatabase(t)
	r, err := New(runtime)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	i := admitted(t, r, "db-lease", time.Now().UTC())
	lease, err := r.Claim(ctx, i.ID)
	if err != nil || lease.IsZero() {
		t.Fatalf("claim=%s %v", lease, err)
	}
	other, err := r.Claim(ctx, i.ID)
	if err != nil || !other.IsZero() {
		t.Fatalf("concurrent claim=%s %v", other, err)
	}
	remaining, err := r.PermitCreate(ctx, i.ID, lease)
	if err != nil || remaining <= 0 || remaining > 15*time.Second {
		t.Fatalf("permit=%s %v", remaining, err)
	}
	if _, err = r.PermitCreate(ctx, i.ID, lease.Add(-time.Microsecond)); !errors.Is(err, d.ErrPending) {
		t.Fatalf("foreign lease=%v", err)
	}
	// A consumed/changed lease never grants a new provider mutation.
	if err = owner.Exec("UPDATE public.registration_intents SET lease_until=clock_timestamp()-interval '1 second' WHERE id=?", i.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = r.PermitCreate(ctx, i.ID, lease); !errors.Is(err, d.ErrPending) {
		t.Fatalf("stale lease=%v", err)
	}
	newLease, err := r.Claim(ctx, i.ID)
	if err != nil || newLease.Equal(lease) || newLease.IsZero() {
		t.Fatalf("reclaim=%s %v", newLease, err)
	}
	if _, err = r.PermitCreate(ctx, i.ID, lease); !errors.Is(err, d.ErrPending) {
		t.Fatalf("old owner=%v", err)
	}
}

func ownedDatabase(t *testing.T) (*gorm.DB, *gorm.DB) {
	t.Helper()
	ctx := context.Background()
	password := uuid.NewString()
	c, err := pg.Run(ctx, "postgres:16-alpine", pg.WithDatabase("referrals"), pg.WithUsername("owner"), pg.WithPassword(password), pg.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(context.Background()); err != nil {
			t.Error(err)
		}
	})
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	open := func(dsn string) *gorm.DB {
		db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if e != nil {
			t.Fatal("owned PG connect failed")
		}
		pool, e := db.DB()
		if e != nil {
			t.Fatal(e)
		}
		pool.SetMaxOpenConns(8)
		t.Cleanup(func() {
			if e := pool.Close(); e != nil {
				t.Error(e)
			}
		})
		return db
	}
	owner := open(dsn)
	if err = Install(ctx, owner); err != nil {
		t.Fatal(err)
	}
	if e := owner.Exec(`CREATE ROLE referral_runtime LOGIN PASSWORD '` + password + `'; REVOKE ALL ON SCHEMA public FROM PUBLIC; GRANT USAGE ON SCHEMA public TO referral_runtime; REVOKE CREATE,TEMP ON DATABASE referrals FROM PUBLIC; GRANT CONNECT ON DATABASE referrals TO referral_runtime; GRANT SELECT,INSERT ON ALL TABLES IN SCHEMA public TO referral_runtime; GRANT SELECT,INSERT,UPDATE ON TABLE public.referral_earning_claims, public.referral_earnings_projection, public.referral_withdrawals TO referral_runtime; GRANT UPDATE(state,ciphertext,lease_until) ON public.registration_intents TO referral_runtime; GRANT UPDATE,DELETE ON public.registration_admission_buckets TO referral_runtime`).Error; e != nil {
		t.Fatal(e)
	}
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "5432/tcp")
	runtime := open(fmt.Sprintf("host=%s port=%s user=referral_runtime password=%s dbname=referrals sslmode=disable", host, port.Port(), password))
	return owner, runtime
}

// The fixture uses sslmode=disable. Suppress the server's completed COMMIT frame
// after it executes, so database/sql sees EOF while the durable transaction exists.
type lostCommitConnection struct {
	net.Conn
	drop    *atomic.Bool
	pending []byte
}

func (c *lostCommitConnection) Read(out []byte) (int, error) {
	if len(c.pending) == 0 {
		header := make([]byte, 5)
		if _, err := io.ReadFull(c.Conn, header); err != nil {
			return 0, err
		}
		size := int(binary.BigEndian.Uint32(header[1:]))
		if size < 4 || size > 1024*1024 {
			return 0, io.ErrUnexpectedEOF
		}
		payload := make([]byte, size-4)
		if _, err := io.ReadFull(c.Conn, payload); err != nil {
			return 0, err
		}
		if header[0] == 'C' && bytes.Equal(payload, []byte("COMMIT\x00")) && c.drop.CompareAndSwap(true, false) {
			_ = c.Conn.Close()
			return 0, io.EOF
		}
		c.pending = append(header, payload...)
	}
	n := copy(out, c.pending)
	c.pending = c.pending[n:]
	return n, nil
}

func TestPostgresWireCommitAcknowledgementLossReplaysReceipt(t *testing.T) {
	_, runtime := ownedDatabase(t)
	config, err := pgx.ParseConfig(runtime.Dialector.(*postgres.Dialector).DSN)
	if err != nil {
		t.Fatal("fixture config invalid")
	}
	var drop atomic.Bool
	config.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &lostCommitConnection{Conn: conn, drop: &drop}, nil
	}
	pool := stdlib.OpenDB(*config)
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = pool.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("fault connection failed")
	}
	r, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	ctx := context.Background()
	i := admitted(t, r, "commit-loss", now)
	if err = r.Created(ctx, i.ID); err != nil {
		t.Fatal(err)
	}
	drop.Store(true)
	if _, err = r.Consume(ctx, i, now); !errors.Is(err, d.ErrUnknown) {
		t.Fatalf("lost COMMIT acknowledgement=%v", err)
	}
	observer, err := New(runtime)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := observer.Find(ctx, "id", "issuer", i.ID)
	if err != nil || saved.State != "CONSUMED" || len(saved.Ciphertext) != 0 {
		t.Fatal("server did not commit before acknowledgement loss")
	}
	receipt, err := observer.Consume(ctx, i, now.Add(25*time.Hour))
	if err != nil || receipt.IntentID != i.ID {
		t.Fatalf("committed receipt replay=%v", err)
	}
	projection, err := observer.Read(ctx, "issuer", "referrer")
	if err != nil || projection.Count != 1 {
		t.Fatalf("double attribution after unknown=%d %v", projection.Count, err)
	}
	second := i
	second.ID = uuid.NewString()
	second.Subject = uuid.NewString()
	second.KeyHash = "second"
	second.EmailHash = "second"
	drop.Store(true)
	if _, err = r.Admit(ctx, second, "code"); !errors.Is(err, d.ErrUnknown) {
		t.Fatalf("lost admission COMMIT=%v", err)
	}
	retried := second
	retried.ID = uuid.NewString()
	retried.Subject = uuid.NewString()
	recovered, err := observer.Admit(ctx, retried, "code")
	if err != nil || recovered.ID != second.ID || recovered.Subject != second.Subject {
		t.Fatalf("unknown admission replaced original identity: %v", err)
	}
}

func TestConcurrentAdmissionAndConsumptionHaveSingleFact(t *testing.T) {
	_, db := ownedDatabase(t)
	r, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	i := admitted(t, r, "concurrent", now)
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			retry := i
			retry.ID = uuid.NewString()
			retry.Subject = uuid.NewString()
			out, e := r.Admit(ctx, retry, "code")
			if e != nil || out.ID != i.ID || out.Subject != i.Subject {
				t.Errorf("concurrent admission changed identity: %v", e)
			}
		}()
	}
	wg.Wait()
	if err = r.Created(ctx, i.ID); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, e := r.Consume(ctx, i, now)
			if e != nil || out.IntentID != i.ID {
				t.Errorf("concurrent consumption=%v", e)
			}
		}()
	}
	wg.Wait()
	p, err := r.Read(ctx, "issuer", "referrer")
	if err != nil || p.Count != 1 {
		t.Fatalf("concurrent count=%d err=%v", p.Count, err)
	}
}

func TestConsumptionCannotCrossExpiryWhileWaitingForLock(t *testing.T) {
	owner, db := ownedDatabase(t)
	r, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	i := admitted(t, r, "lock-expiry", now.Add(-24*time.Hour+time.Second))
	if err = r.Created(ctx, i.ID); err != nil {
		t.Fatal(err)
	}
	lock := owner.Begin()
	if lock.Error != nil {
		t.Fatal(lock.Error)
	}
	defer lock.Rollback()
	if err = lock.Exec("SELECT id FROM public.registration_intents WHERE id=? FOR UPDATE", i.ID).Error; err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { _, e := r.Consume(ctx, i, now); result <- e }()
	time.Sleep(time.Until(i.CompletionExpiresAt.Add(100 * time.Millisecond)))
	if err = lock.Commit().Error; err != nil {
		t.Fatal(err)
	}
	if err = <-result; !errors.Is(err, d.ErrExpired) {
		t.Fatalf("consumed after waiting past expiry: %v", err)
	}
}

func admitted(t *testing.T, r *Repository, key string, now time.Time) d.Intent {
	t.Helper()
	if _, err := r.CreateCode(context.Background(), "issuer", "referrer", "code"); err != nil {
		t.Fatal(err)
	}
	i, err := r.Admit(context.Background(), d.Intent{ID: uuid.NewString(), Issuer: "issuer", Instance: "instance", Organization: "signup", Subject: uuid.NewString(), KeyHash: key, EmailHash: key, Fingerprint: key, SecretHash: "hash", KeyID: "v1", Ciphertext: []byte("sealed"), CreatedAt: now, CreateExpiresAt: now.Add(15 * time.Minute), CompletionExpiresAt: now.Add(24 * time.Hour), State: "PREPARED"}, "code")
	if err != nil {
		t.Fatal(err)
	}
	return i
}
func TestConsumptionFailureRollsBackRelationReceiptAndWipe(t *testing.T) {
	owner, db := ownedDatabase(t)
	r, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	i := admitted(t, r, "atomic", now)
	if err = r.Created(ctx, i.ID); err != nil {
		t.Fatal(err)
	}
	if err = owner.Exec(`CREATE FUNCTION public.reject_receipt() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RAISE EXCEPTION ''controlled receipt failure''; END'; CREATE TRIGGER reject_receipt BEFORE INSERT ON public.referral_receipts FOR EACH ROW EXECUTE FUNCTION public.reject_receipt()`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = r.Consume(ctx, i, now); err == nil {
		t.Fatal("fault must fail consumption")
	}
	saved, err := r.Find(ctx, "id", "issuer", i.ID)
	if err != nil || saved.State != "CREATED" || len(saved.Ciphertext) == 0 {
		t.Fatal("transaction did not roll back intent")
	}
	p, err := r.Read(ctx, "issuer", "referrer")
	if err != nil || p.Count != 0 {
		t.Fatal("transaction leaked relation")
	}
	if _, err = r.Receipt(ctx, "issuer", i.Subject); !errors.Is(err, d.ErrMissing) {
		t.Fatalf("transaction leaked receipt %v", err)
	}
	if err = owner.Exec("DROP TRIGGER reject_receipt ON public.referral_receipts").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = r.Consume(ctx, i, now); err != nil {
		t.Fatal(err)
	}
	i.Referrer = "other"
	if _, err = r.Consume(ctx, i, now); !errors.Is(err, d.ErrConflict) {
		t.Fatalf("different relation replay=%v", err)
	}
}
func TestExpiryCleanupIsBoundedAndRetainsIdentityKeys(t *testing.T) {
	_, db := ownedDatabase(t)
	r, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	for n := 0; n < 23; n++ {
		admitted(t, r, fmt.Sprintf("expired-%d", n), now.Add(-25*time.Hour))
	}
	live := admitted(t, r, "live", now)
	if err = r.Cleanup(ctx, now); err != nil {
		t.Fatal(err)
	}
	var total, wiped int64
	db.Table("public.registration_intents").Count(&total)
	db.Table("public.registration_intents").Where("ciphertext IS NULL").Count(&wiped)
	if total != 24 || wiped != 20 {
		t.Fatalf("cleanup total=%d wiped=%d", total, wiped)
	}
	saved, err := r.Find(ctx, "id", "issuer", live.ID)
	if err != nil || len(saved.Ciphertext) == 0 {
		t.Fatal("live payload erased")
	}
	if err = r.Cleanup(ctx, now); err != nil {
		t.Fatal(err)
	}
	db.Table("public.registration_intents").Where("ciphertext IS NULL").Count(&wiped)
	if wiped != 23 {
		t.Fatalf("second bounded pass=%d", wiped)
	}
}
func TestPostgresRejectOwnerPrivileges(t *testing.T) {
	owner, runtime := ownedDatabase(t)
	if VerifyPermissions(context.Background(), owner) == nil {
		t.Fatal("owner must not pass runtime permission inventory")
	}
	if e := VerifyPermissions(context.Background(), runtime); e != nil {
		t.Fatal(e)
	}
}
func TestPostgresAdmissionAndAtomicConsumption(t *testing.T) {
	_, db := ownedDatabase(t)
	ctx := context.Background()
	r, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.CreateCode(ctx, "issuer", "referrer", "code"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	i := d.Intent{ID: uuid.NewString(), Issuer: "issuer", Instance: "instance", Organization: "signup", Subject: uuid.NewString(), KeyHash: "key", EmailHash: "email", Fingerprint: "payload", SecretHash: "hash", KeyID: "v1", Ciphertext: []byte("sealed"), CreatedAt: now, CreateExpiresAt: now.Add(15 * time.Minute), CompletionExpiresAt: now.Add(24 * time.Hour), State: "PREPARED"}
	a, err := r.Admit(ctx, i, "code")
	if err != nil {
		t.Fatal(err)
	}
	i.ID = uuid.NewString()
	i.Subject = uuid.NewString()
	b, err := r.Admit(ctx, i, "code")
	if err != nil || a.ID != b.ID || a.Subject != b.Subject {
		t.Fatalf("replay: %v", err)
	}
	if err = r.Created(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	receipt, err := r.Consume(ctx, a, now)
	if err != nil {
		t.Fatal(err)
	}
	again, err := r.Consume(ctx, a, now.Add(48*time.Hour))
	if err != nil || again != receipt {
		t.Fatalf("successful late replay: %v", err)
	}
	projection, err := r.Read(ctx, "issuer", "referrer")
	if err != nil || projection.Count != 1 || projection.EarningsAvailability != "unavailable" {
		t.Fatalf("projection=%+v %v", projection, err)
	}
	saved, err := r.Find(ctx, "id", "issuer", a.ID)
	if err != nil || len(saved.Ciphertext) != 0 || saved.State != "CONSUMED" {
		t.Fatalf("consume not atomic: %+v %v", saved, err)
	}
}
