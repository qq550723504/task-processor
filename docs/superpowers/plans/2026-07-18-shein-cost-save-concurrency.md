# SHEIN Cost Save Concurrency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep rapid SHEIN cost maintenance responsive by coalescing candidate-cost rebuilds and preventing whole-store UI refresh storms.

**Architecture:** The sync service records a manual cost synchronously, then passes a scope key to a small in-memory coalescing refresher. The UI serializes saves inside the workbench and refreshes only the source-cost-group data after a successful save.

**Tech Stack:** Go, GORM repository interfaces, React, TanStack Query, Vitest.

## Global Constraints

- Preserve durable manual-cost writes before returning HTTP 200.
- Do not introduce a database migration or external queue.
- Candidate refresh failures are observable in logs and do not roll back a successful cost edit.
- Use `GOWORK=off` for Go commands from a nested project worktree.

---

### Task 1: Coalesce asynchronous candidate-cost rebuilds

**Files:**
- Create: `internal/listingkit/sheinsync/cost_refresh_coordinator.go`
- Create: `internal/listingkit/sheinsync/cost_refresh_coordinator_test.go`
- Modify: `internal/listingkit/sheinsync/service_sync.go`

**Interfaces:**
- Produces: `costRefreshCoordinator.Schedule(key string, refresh func(context.Context) error)`.
- Consumes: `refreshCandidateCostsForSDSCostGroup` from the sync service.

- [ ] **Step 1: Write failing tests**

```go
func TestCostRefreshCoordinatorCoalescesSameScope(t *testing.T) {
    started := make(chan struct{})
    release := make(chan struct{})
    var runs atomic.Int32
    coordinator := newCostRefreshCoordinator(context.Background(), func(error) {})
    coordinator.Schedule("227:870:source:A", func(context.Context) error {
        runs.Add(1); close(started); <-release; return nil
    })
    <-started
    coordinator.Schedule("227:870:source:A", func(context.Context) error { runs.Add(1); return nil })
    close(release)
    require.Eventually(t, func() bool { return runs.Load() == 2 }, time.Second, time.Millisecond)
}
```

- [ ] **Step 2: Run the test and confirm RED**

Run: `$env:GOWORK='off'; go test ./internal/listingkit/sheinsync -run TestCostRefreshCoordinatorCoalescesSameScope -count=1`

Expected: compilation failure because the coordinator does not exist.

- [ ] **Step 3: Implement the coordinator and wire cost-group saves**

```go
if err := repo.UpsertSDSCostGroup(ctx, row); err != nil { return nil, err }
s.costRefreshes.Schedule(costRefreshScopeKey(tenantID, storeID, groupKey), func(refreshCtx context.Context) error {
    return s.refreshCandidateCostsForSDSCostGroup(refreshCtx, repo, tenantID, storeID, groupKey)
})
return s.loadSDSCostGroup(ctx, repo, tenantID, storeID, groupKey, row)
```

- [ ] **Step 4: Run focused tests and confirm GREEN**

Run: `$env:GOWORK='off'; go test ./internal/listingkit/sheinsync -run 'TestCostRefreshCoordinator|TestUpdateSDSCostGroupManualCost' -count=1`

Expected: PASS.

### Task 2: Avoid whole-store invalidation after each cost save

**Files:**
- Modify: `web/listingkit-ui/src/lib/query/use-shein-enrollment.ts`
- Modify: `web/listingkit-ui/src/lib/query/use-shein-enrollment.test.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-products/shein-products-store-workbench.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-products/shein-cost-price-table.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein-products/shein-cost-price-table.test.tsx`

**Interfaces:**
- Consumes: the two cost mutations.
- Produces: sequential cost-save dispatch and cost-query-only invalidation.

- [ ] **Step 1: Write a failing mutation test**

```tsx
expect(queryClient.invalidateQueries).toHaveBeenCalledWith({
  queryKey: listingKitKeys.sheinEnrollmentSourceSDSCostGroups(12),
});
expect(queryClient.invalidateQueries).not.toHaveBeenCalledWith({
  queryKey: listingKitKeys.sheinEnrollmentStoreScope(12),
});
```

- [ ] **Step 2: Run the focused test and confirm RED**

Run: `npm --prefix web/listingkit-ui test -- --run src/lib/query/use-shein-enrollment.test.tsx`

Expected: assertion shows whole-store scope invalidation.

- [ ] **Step 3: Implement narrow invalidation and serialized dispatch**

```ts
onSuccess: () => client.invalidateQueries({
  queryKey: listingKitKeys.sheinEnrollmentSourceSDSCostGroups(storeId),
})
```

Use one promise tail in the workbench so each `saveSheinCostTarget` starts after the prior save settles; keep row-level errors attached to the initiating row.

- [ ] **Step 4: Run focused UI tests and confirm GREEN**

Run: `npm --prefix web/listingkit-ui test -- --run src/lib/query/use-shein-enrollment.test.tsx src/components/listingkit/shein-products/shein-cost-price-table.test.tsx`

Expected: PASS.

### Task 3: End-to-end verification

**Files:**
- Modify: no production files beyond Tasks 1 and 2.

- [ ] **Step 1: Run backend suite**

Run: `$env:GOWORK='off'; go test ./internal/listingkit/sheinsync -count=1 -timeout 5m`

Expected: PASS.

- [ ] **Step 2: Run frontend typecheck and focused tests**

Run: `npm --prefix web/listingkit-ui run typecheck`

Expected: PASS.

- [ ] **Step 3: Inspect diff and commit**

Run: `git diff --check; git status --short`

Commit: `fix(shein): coalesce cost maintenance refreshes`
