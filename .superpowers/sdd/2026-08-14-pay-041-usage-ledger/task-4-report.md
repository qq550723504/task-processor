# PAY-041 Task 4: in-memory usage ledger report

## Scope

- Added `NewMemUsageLedger(*MemRepository) UsageLedger` in `internal/listingsubscription/usage_ledger_mem.go`.
- Added mutex-protected event identity, bucket, outbox, and reversal state. The implementation preserves the GORM ledger's reservation, commit, release, reversal, idempotency, quota, entitlement, and `int64` overflow semantics for in-memory service and handler tests.
- Added concurrency tests in `internal/listingsubscription/usage_ledger_mem_test.go`; no legacy aggregate-counter behavior, provider calls, payment behavior, or PAY-042 cutover was changed.

## TDD evidence

The concurrency tests were written before the ledger implementation.

1. RED:

   ```text
   go test -race ./internal/listingsubscription -run 'TestMemUsageLedger|TestUsageLedgerConcurrent' -count=1
   FAIL: undefined: NewMemUsageLedger
   ```

2. GREEN:

   ```text
   go test -race ./internal/listingsubscription -run 'TestMemUsageLedger|TestUsageLedgerConcurrent' -count=1
   PASS
   ```

The tests start 20 goroutines with distinct identities against a limit of 10, verify exactly 10 reservations and 10 distinct event/outbox identities, and concurrently replay a single identity 20 times while proving one event and reservation.

## Verification

```text
go test -race ./internal/listingsubscription -run 'TestMemUsageLedger|TestUsageLedgerConcurrent' -count=20
PASS

go test ./internal/listingsubscription -count=1
PASS

go vet ./internal/listingsubscription
PASS

git diff --check
PASS
```

An additional root-module `go test ./...; go vet ./...` attempt reached the 120-second command timeout without test-failure output. It is not claimed as passing evidence; the focused package verification above is complete.
