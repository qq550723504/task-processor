//go:build integration

package commercialbilling

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"testing"
	"time"

	"task-processor/internal/commercial/billing"

	"github.com/google/uuid"
	pg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestTopUpPostgresConcurrentChannelClaimAndDurableInbox(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := pg.Run(ctx, "postgres:16-alpine", pg.WithDatabase("billing"), pg.WithUsername("schema_owner"), pg.WithPassword(uuid.NewString()), pg.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	open := func(dsn string) *gorm.DB {
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			t.Fatal("fixture connection failed")
		}
		pool, _ := db.DB()
		t.Cleanup(func() { _ = pool.Close() })
		return db
	}
	owner := open(dsn)
	if err = AutoMigrate(owner); err != nil {
		t.Fatal(err)
	}
	password := uuid.NewString()
	if err = owner.Exec(`CREATE ROLE commercial_owner_runtime LOGIN PASSWORD '` + password + `'; REVOKE ALL ON SCHEMA public FROM PUBLIC; GRANT USAGE ON SCHEMA public TO commercial_owner_runtime;
 GRANT SELECT,INSERT,UPDATE ON public.commercial_orders,public.commercial_topup_attempts,public.commercial_topup_refunds TO commercial_owner_runtime;
 GRANT SELECT,INSERT ON public.commercial_order_items,public.commercial_topup_inbox TO commercial_owner_runtime;`).Error; err != nil {
		t.Fatal(err)
	}
	endpoint, _ := url.Parse(dsn)
	endpoint.User = url.UserPassword("commercial_owner_runtime", password)
	db := open(endpoint.String())
	r, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	successes := make(chan billing.TopUpPaymentAttempt, 8)
	failures := make(chan error, 8)
	now := time.Now().UTC().Truncate(time.Microsecond)
	for i := range 8 {
		wg.Go(func() {
			provider := billing.PaymentAlipay
			if i%2 == 0 {
				provider = billing.PaymentWeChat
			}
			a, err := r.CreateTopUpAttempt(ctx, billing.CreateWalletTopUpOrderRequest{OrganizationID: "org-1", ActorID: "admin-1", IdempotencyKey: "same-intent", Provider: provider, AmountMinor: 10000, Currency: "CNY"}, topUpMerchant(provider), now, now.Add(10*time.Minute))
			if err != nil {
				failures <- err
				return
			}
			successes <- a
		})
	}
	wg.Wait()
	close(successes)
	close(failures)
	for err := range failures {
		if !errors.Is(err, billing.ErrConflict) {
			t.Fatal(err)
		}
	}
	var first billing.TopUpPaymentAttempt
	count := 0
	for a := range successes {
		count++
		if first.OrderID == "" {
			first = a
		}
		if a.OrderID != first.OrderID || a.Merchant.Provider != first.Merchant.Provider {
			t.Fatal("same key created two channels")
		}
	}
	if count != 4 {
		t.Fatalf("same-channel replays = %d", count)
	}
	var orders, attempts int64
	owner.Model(&orderRow{}).Count(&orders)
	owner.Model(&topUpAttemptRow{}).Count(&attempts)
	if orders != 1 || attempts != 1 {
		t.Fatalf("partial/duplicate B1: orders=%d attempts=%d", orders, attempts)
	}
	o := billing.ProviderObservation{Merchant: first.Merchant, MerchantOrderID: first.MerchantOrderID, EventID: "same-event", Kind: "PAYMENT", State: "PAID", TradeID: "trade-1", Currency: "CNY", AmountMinor: 10000, OccurredAt: now, VerificationVersion: "fixture-v1"}
	errs := make(chan error, 6)
	for range 6 {
		wg.Go(func() { errs <- r.RecordTopUpObservation(ctx, o) })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	restarted, _ := New(open(endpoint.String()))
	saved, err := restarted.ReadTopUpAttempt(ctx, "org-1", first.OrderID)
	if err != nil || !saved.NeedsReconcile || saved.Version != first.Version+1 {
		t.Fatalf("durable wakeup: %+v %v", saved, err)
	}
	evidence, err := restarted.ReadTopUpObservations(ctx, saved)
	if err != nil || len(evidence) != 1 {
		t.Fatalf("durable evidence: %d %v", len(evidence), err)
	}
	if err = db.Exec("UPDATE commercial_topup_inbox SET fingerprint='rewritten'").Error; err == nil {
		t.Fatal("runtime may rewrite verified inbox")
	}
}
