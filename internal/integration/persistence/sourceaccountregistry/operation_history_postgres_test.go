//go:build integration

package sourceaccountregistry_test

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	schema "task-processor/internal/app/schema/sourceaccountregistry"
	store "task-processor/internal/integration/persistence/sourceaccountregistry"
	registry "task-processor/internal/sourceaccountregistry"
)

func TestAccountAuditCommittedHistoryPostgres(t *testing.T) {
	db, dsn := openRegistryPostgresWithDSN(t) // Fresh container/pool owned and cleaned by this test.
	ctx := context.Background()
	if err := schema.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	service := newRegistryService(t, db, now)
	operator := registryIdentity(now, "B", "actor-B", "listingkit_operator")
	key := uuid.NewString()
	first, err := service.Register(operator, key, registry.RegisterInput{DisplayName: "not in audit", Platform: "1688"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Disable(operator, uuid.NewString(), first.Account.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Enable(operator, uuid.NewString(), first.Account.ID, 2); err != nil {
		t.Fatal(err)
	}
	second, err := service.Register(operator, uuid.NewString(), registry.RegisterInput{DisplayName: "second", Platform: "1688"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Register(registryIdentity(now, "A", "actor-A", "listingkit_operator"), uuid.NewString(), registry.RegisterInput{DisplayName: "foreign", Platform: "1688"}); err != nil {
		t.Fatal(err)
	}
	repo, err := store.NewRepository(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	before, err := repo.ListCommittedOperations(ctx, "B", registry.HistoryRequest{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Items) != 4 {
		t.Fatalf("history: %+v", before)
	}
	for _, item := range before.Items {
		if item.OrganizationID != "B" || item.ActorSubject != "actor-B" || !item.OccurredAt.Equal(now) {
			t.Fatalf("fact: %+v", item)
		}
	}
	replay, err := service.Register(operator, key, registry.RegisterInput{DisplayName: "not in audit", Platform: "1688"})
	if err != nil || !replay.Replayed {
		t.Fatalf("replay %v", err)
	}
	if _, err = service.Disable(operator, uuid.NewString(), first.Account.ID, 1); !errors.Is(err, registry.ErrVersionConflict) {
		t.Fatal(err)
	}
	// Even Complete followed by a failed callback must remain invisible.
	rollbackID, _ := uuid.NewV7()
	account, _ := registry.NewAccount(rollbackID.String(), "B", "actor-B", "rollback", registry.Platform1688, now)
	_, err = repo.Run(ctx, registry.Operation{Scope: registry.Scope{OrganizationID: "B", ActorSubject: "actor-B"}, Key: uuid.NewString(), Kind: registry.OperationRegister, Fingerprint: strings.Repeat("a", 64)}, func(tx registry.Transaction) (registry.Account, error) {
		if _, e := tx.CountForCreate(); e != nil {
			return registry.Account{}, e
		}
		if e := tx.Insert(account); e != nil {
			return registry.Account{}, e
		}
		if e := tx.Complete(account); e != nil {
			return registry.Account{}, e
		}
		return registry.Account{}, errors.New("rollback")
	})
	if err == nil {
		t.Fatal("rollback succeeded")
	}
	after, err := repo.ListCommittedOperations(ctx, "B", registry.HistoryRequest{Limit: 100})
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("immutable/replay/rollback: %v", err)
	}
	// Use a separate connection with every transaction forced read-only.
	parsed, _ := url.Parse(dsn)
	query := parsed.Query()
	query.Set("default_transaction_read_only", "on")
	parsed.RawQuery = query.Encode()
	readonly := openRegistryGORM(t, parsed.String())
	readRepo, err := store.NewRepository(ctx, readonly)
	if err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int{1, 2, 3, 100} {
		var got []registry.CommittedOperation
		var cursor *registry.HistoryPosition
		for pageNumber := 0; pageNumber < 6; pageNumber++ {
			page, e := readRepo.ListCommittedOperations(ctx, "B", registry.HistoryRequest{Limit: limit, After: cursor})
			if e != nil {
				t.Fatal(e)
			}
			got = append(got, page.Items...)
			if page.Next == nil {
				break
			}
			cursor = page.Next
		}
		if !reflect.DeepEqual(got, before.Items) {
			t.Fatalf("limit %d lost/reordered receipts: %+v", limit, got)
		}
	}
	empty, err := readRepo.ListCommittedOperations(ctx, "empty-org", registry.HistoryRequest{Limit: 20})
	if err != nil || len(empty.Items) != 0 || empty.Next != nil {
		t.Fatalf("empty: %+v %v", empty, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = readRepo.ListCommittedOperations(canceled, "B", registry.HistoryRequest{Limit: 20}); err == nil {
		t.Fatal("canceled query succeeded")
	}
	// A lock stalls the actual SELECT. The caller deadline must interrupt it.
	lock := db.Begin()
	if lock.Error != nil {
		t.Fatal(lock.Error)
	}
	defer lock.Rollback()
	if err = lock.Exec("LOCK TABLE public.source_account_operations IN ACCESS EXCLUSIVE MODE").Error; err != nil {
		t.Fatal(err)
	}
	short, stop := context.WithTimeout(ctx, 150*time.Millisecond)
	defer stop()
	started := time.Now()
	if _, err = readRepo.ListCommittedOperations(short, "B", registry.HistoryRequest{Limit: 20}); err == nil || time.Since(started) > 3*time.Second {
		t.Fatalf("deadline not enforced: %v", err)
	}
	if err = lock.Rollback().Error; err != nil {
		t.Fatal(err)
	}
	if _, err = readRepo.ListCommittedOperations(ctx, "B", registry.HistoryRequest{Limit: 101}); !errors.Is(err, registry.ErrInvalid) {
		t.Fatal(err)
	}
	// Query failure is unavailable, never an empty success.
	sqlDB, _ := readonly.DB()
	if err = sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = readRepo.ListCommittedOperations(ctx, "B", registry.HistoryRequest{Limit: 20}); !errors.Is(err, registry.ErrUnavailable) {
		t.Fatalf("closed pool: %v", err)
	}
	t.Logf("source-owned receipts: 4; same-microsecond versions/account tie; account IDs %s %s; readonly SELECT and canceled/blocked SELECT exercised", first.Account.ID, second.Account.ID)
}
