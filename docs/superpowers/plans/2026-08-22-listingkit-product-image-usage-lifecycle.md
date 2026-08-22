# ListingKit Product Image Usage Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make batch product-image accounting route and reservation ownership durable so rollout changes and synchronous recovery cannot corrupt metering.

**Architecture:** AI batch links start with a durable `pending` accounting-route sentinel. The creating worker resolves it once to `ledger` or `legacy`, and all later authorization, settlement, and release operations read that stored route. Ledger events use distinct source types for synchronous API, async jobs, and batch work; reconciliation expires only source types that prove synchronous API ownership.

**Tech Stack:** Go, GORM, SQLite test databases, in-memory repositories, Go testing and race detector.

**Spec:** `docs/superpowers/specs/2026-08-22-listingkit-product-image-usage-lifecycle-design.md`

## Global Constraints

- Do not re-evaluate `generationUsageAdmission` after a batch link has a final accounting route.
- New AI links store `pending`; an empty route is compatibility-only for pre-change rows.
- Only direct synchronous reservations use the 30-minute expiry lease.
- Preserve legacy mirror release-pending semantics until its compensating counter operation succeeds.
- Keep changes scoped to ListingKit batch links, product-image usage adapters, and reconciliation.

---

### Task 1: Persist and atomically resolve the batch accounting route

**Files:**
- Modify: `internal/listingkit/studio_batch_task_link_model.go`
- Modify: `internal/listingkit/studio_batch_task_link_repository.go`
- Modify: `internal/listingkit/studio_batch_task_link_repository_gorm.go`
- Modify: `internal/listingkit/studio_batch_task_link_repository_mem.go`
- Modify: `internal/listingkit/studio_batch_task_link_repository_test.go`

**Interfaces:**
- Produces `studioBatchProductImageUsageRoute` with `pending`, `legacy`, and `ledger` values.
- Produces:

```go
type studioBatchTaskLinkProductImageUsageRouteRepository interface {
    ResolveStudioBatchProductImageUsageRoute(
        ctx context.Context,
        candidateKey string,
        claimToken string,
        route studioBatchProductImageUsageRoute,
        updatedAt time.Time,
    ) (stored studioBatchProductImageUsageRoute, changed bool, err error)
}
```

- [ ] **Step 1: Write the failing repository parity tests**

In `studio_batch_task_link_repository_test.go`, add a shared Mem/GORM helper that creates a `creating` link with `ProductImageUsageRoute: pending`, resolves it to `ledger`, then checks one immutable final value:

```go
stored, changed, err := resolver.ResolveStudioBatchProductImageUsageRoute(
    ctx, link.CandidateKey, link.ClaimToken,
    studioBatchProductImageUsageRouteLedger, now.Add(time.Minute),
)
if err != nil || !changed || stored != studioBatchProductImageUsageRouteLedger {
    t.Fatalf("route resolution = (%q, %v, %v)", stored, changed, err)
}
```

Add cases proving a second `legacy` attempt returns the stored `ledger` route unchanged and a mismatched claim token cannot resolve `pending`.

- [ ] **Step 2: Run the test and observe the expected red failure**

Run:

```powershell
go test ./internal/listingkit -run 'Test(Mem|Gorm)StudioBatchTaskLinkRepositoryResolvesProductImageUsageRoute$' -count=1
```

Expected: compile failure because the model field and resolver contract do not exist.

- [ ] **Step 3: Implement the minimal durable route boundary**

Add:

```go
ProductImageUsageRoute string `json:"-" gorm:"column:product_image_usage_route;type:varchar(16);index"`
```

Add a GORM ensure/migration helper and include the column in every complete-link update map and Mem copy/update path. Implement the resolver with a conditional update that requires `status=creating`, the exact claim token, and `route=pending`; if no row changes, read the scoped link and return its final route, or return lease loss when it remains pending under another owner.

- [ ] **Step 4: Run the test and observe green**

Run:

```powershell
go test ./internal/listingkit -run 'Test(Mem|Gorm)StudioBatchTaskLinkRepository.*ProductImageUsageRoute' -count=1
```

Expected: Mem and GORM persist one immutable route and reject stale ownership.

- [ ] **Step 5: Commit**

```powershell
git add -- internal/listingkit/studio_batch_task_link_model.go internal/listingkit/studio_batch_task_link_repository.go internal/listingkit/studio_batch_task_link_repository_gorm.go internal/listingkit/studio_batch_task_link_repository_mem.go internal/listingkit/studio_batch_task_link_repository_test.go
git commit -m "feat: persist batch product image usage route"
```

### Task 2: Bind batch authorization and settlement to its stored route

**Files:**
- Modify: `internal/listingkit/task_studio_batch_task_flow_support.go`
- Modify: `internal/listingkit/task_studio_batch_product_images.go`
- Modify: `internal/listingkit/usage_settlement.go`
- Modify: `internal/listingkit/httpapi/studio_product_image_usage.go`
- Modify: `internal/listingkit/task_studio_batch_service_request_test.go`
- Modify: `internal/listingkit/httpapi/studio_product_image_usage_test.go`

**Interfaces:**
- Consumes the route resolver from Task 1.
- Produces:

```go
type StudioProductImageUsageReservationLookup interface {
    HasProductImageUsageReservation(context.Context, string, string) (bool, error)
}
```

- `subscriptionStudioProductImageUsage` implements this lookup using its deterministic idempotency key; an absent event returns `false, nil`.

- [ ] **Step 1: Write the failing rollout-transition tests**

Add tests named:

```go
func TestTaskStudioBatchServiceSettlesPersistedLegacyRouteAfterTenantEntersLedger(t *testing.T)
func TestTaskStudioBatchServiceSettlesPersistedLedgerRouteAfterTenantLeavesLedger(t *testing.T)
func TestTaskStudioBatchServiceUsesLegacyRouteForPreChangeLinkWithoutReservation(t *testing.T)
```

The first test authorizes while a mutable admission stub denies ledger, flips the stub to allow before settlement, and asserts one legacy record and zero reservation commits. The second starts with ledger allowed, flips to deny, and asserts one commit with zero legacy record. The third uses an empty pre-change route and lookup `false`, flips admission to allow, and asserts durable legacy settlement.

- [ ] **Step 2: Run the tests and observe red**

Run:

```powershell
go test ./internal/listingkit -run 'TestTaskStudioBatchService(SettlesPersisted.*Route|UsesLegacyRouteForPreChangeLinkWithoutReservation)$' -count=1
```

Expected: current settlement re-evaluates admission, causing at least one transition assertion to fail.

- [ ] **Step 3: Implement route selection once and consume it everywhere**

When an AI link is initially created in `reserveStudioBatchTaskCandidate`, set its route to `pending`. Before authorization, resolve pending with the current admission and reservation availability under the candidate claim token. Persist the final route before calling either authorization path. For an empty old route, query `HasProductImageUsageReservation`: persist `ledger` if found, otherwise persist `legacy`; do not inspect the rollout predicate in this compatibility branch.

Refactor `authorizeStudioBatchProductImageUsage`, `settleStudioBatchProductImageUsage`, and batch release to switch on the stored final route. Remove their direct admission checks after route resolution. Have `subscriptionStudioProductImageUsage.ReserveProductImageUsage` produce `listingkit_batch_product_image` while retaining its deterministic idempotency key.

- [ ] **Step 4: Run the tests and observe green**

Run:

```powershell
go test ./internal/listingkit ./internal/listingkit/httpapi -run 'TestTaskStudioBatchService.*ProductImage|TestSubscriptionStudioProductImageUsage' -count=1
```

Expected: both rollout transitions use their original accounting system and batch events have the batch source type.

- [ ] **Step 5: Commit**

```powershell
git add -- internal/listingkit/task_studio_batch_task_flow_support.go internal/listingkit/task_studio_batch_product_images.go internal/listingkit/usage_settlement.go internal/listingkit/httpapi/studio_product_image_usage.go internal/listingkit/task_studio_batch_service_request_test.go internal/listingkit/httpapi/studio_product_image_usage_test.go
git commit -m "fix: bind batch image usage to its authorized route"
```

### Task 3: Separate reconciliation ownership and protect historical events

**Files:**
- Modify: `internal/listingkit/api/studio_product_image_usage_admission.go`
- Modify: `internal/listingkit/api/studio_product_image_usage_admission_test.go`
- Modify: `internal/listingkit/httpapi/studio_product_image_usage.go`
- Modify: `internal/listingkit/httpapi/studio_product_image_usage_test.go`

**Interfaces:**
- Consumes `listingkit_batch_product_image` from Task 2.
- Produces source predicates that distinguish new sync, async, batch, old API sync, and ambiguous historical events.

- [ ] **Step 1: Write failing reconciliation boundary tests**

Add:

```go
func TestReconcileStudioProductImageUsageKeepsExpiredBatchReservation(t *testing.T)
func TestReconcileStudioProductImageUsageReleasesExpiredSynchronousReservation(t *testing.T)
func TestReconcileStudioProductImageUsageReleasesOldGenericDirectReservation(t *testing.T)
func TestReconcileStudioProductImageUsageKeepsAmbiguousOldGenericReservation(t *testing.T)
```

Create all events two hours old. Use `listingkit_batch_product_image` for a batch, `listingkit_sync_product_image` for new direct sync, and old `listingkit_product_image` with respectively `listingkit:api:studio_product_image:` and `listingkit:studio_product_image:` idempotency prefixes. Assert event status and legacy mirror quantity after reconciliation.

- [ ] **Step 2: Run the tests and observe red**

Run:

```powershell
go test ./internal/listingkit/api -run 'TestReconcileStudioProductImageUsage.*(BatchReservation|SynchronousReservation|Generic)' -count=1
```

Expected: the current generic-source expiry branch releases the batch reservation.

- [ ] **Step 3: Restrict expiry to synchronous ownership**

Create direct reservations with `listingkit_sync_product_image`. Let reconciliation query sync, async, batch, and old generic source types, but release on age only when:

```go
event.SourceType == studioProductImageSyncSourceType ||
(event.SourceType == studioProductImageLegacySourceType &&
    strings.HasPrefix(event.IdempotencyKey, "listingkit:api:studio_product_image:"))
```

Keep batch events eligible for existing mirror and pending-release repairs but never age-release them. Leave old generic events without the API prefix untouched.

- [ ] **Step 4: Run the tests and observe green**

Run:

```powershell
go test ./internal/listingkit/api ./internal/listingkit/httpapi -count=1
go test -race ./internal/listingkit -run 'TestTaskStudioBatchServiceSettlesPersisted.*Route' -count=1
go test -race ./internal/listingkit/api -run 'TestReconcileStudioProductImageUsage.*(BatchReservation|SynchronousReservation|Generic)' -count=1
```

Expected: only direct synchronous work is lease-recovered; batch and ambiguous historical events remain intact.

- [ ] **Step 5: Commit**

```powershell
git add -- internal/listingkit/api/studio_product_image_usage_admission.go internal/listingkit/api/studio_product_image_usage_admission_test.go internal/listingkit/httpapi/studio_product_image_usage.go internal/listingkit/httpapi/studio_product_image_usage_test.go
git commit -m "fix: isolate product image reservation recovery"
```

### Task 4: Verify and publish the merged-PR follow-up

**Files:**
- Modify: `docs/superpowers/specs/2026-08-22-listingkit-product-image-usage-lifecycle-design.md`
- Modify: `docs/superpowers/plans/2026-08-22-listingkit-product-image-usage-lifecycle.md`

**Interfaces:**
- Consumes the repository, batch-service, adapter, and reconciliation behavior from Tasks 1-3.
- Produces a new reviewable follow-up PR; PR #166 remains merged.

- [ ] **Step 1: Run focused verification**

Run:

```powershell
go test ./internal/listingkit ./internal/listingkit/api ./internal/listingkit/httpapi ./internal/listingsubscription -count=1
git diff --check
git status --short --branch
```

Expected: affected packages pass, no whitespace errors, and only intended branch changes exist.

- [ ] **Step 2: Run repository verification**

Run:

```powershell
go test ./... -count=1
go test ./... -count=1 -p 1
```

Expected: record both outputs. If the parallel command repeats the known intermittent `internal/listingsubscription` failure while serial passes, preserve its full evidence and do not label it a lifecycle regression.

- [ ] **Step 3: Commit documentation adjustments**

```powershell
git add -- docs/superpowers/specs/2026-08-22-listingkit-product-image-usage-lifecycle-design.md docs/superpowers/plans/2026-08-22-listingkit-product-image-usage-lifecycle.md
git commit -m "docs: finalize product image usage lifecycle plan"
```

- [ ] **Step 4: Push and open a draft follow-up PR**

Run:

```powershell
git push -u origin codex/pr166-usage-lifecycle
gh pr create --repo qq550723504/task-processor --base master --head codex/pr166-usage-lifecycle --draft --title "fix: harden product image usage lifecycle"
```

Use the PR body to state the persisted route, source-type isolation, historical-event compatibility, and exact verification results. Do not reopen or push to merged PR #166.

