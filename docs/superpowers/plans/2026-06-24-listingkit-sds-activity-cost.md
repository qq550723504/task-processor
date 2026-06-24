# ListingKit SDS Activity Cost Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make SHEIN activity enrollment candidates calculate with a consistent cost for products that belong to the same SDS base/source group.

**Architecture:** Reuse the SDS identity/source SKU matching pattern introduced by SDS retirement, but apply it inside the SHEIN candidate refresh flow before candidate records are saved. Candidate records remain the cost snapshot used by review and enrollment execution.

**Tech Stack:** Go, GORM models, ListingKit `sheinsync` services, existing SHEIN activity adapter, Go unit tests.

## Global Constraints

- Do not change unrelated ListingKit task behavior.
- Preserve existing manual cost override priority.
- Use existing SDS `source_sds_sku` and SHEIN `SupplierCode` mapping instead of introducing a new external dependency.
- Keep enrollment execution consuming candidate snapshots.

---

### Task 1: SDS Cost Grouping In Candidate Refresh

**Files:**
- Modify: `internal/listingkit/sheinsync/candidate_service.go`
- Modify: `internal/listingkit/sheinsync/candidate_service_test.go`

**Interfaces:**
- Consumes: `SheinSyncedProductRecord.SupplierCode`, `ManualCostPrice`, `EffectiveCostPrice`, and `CostPriceSource`.
- Produces: grouped candidate `EffectiveCostPrice` before `buildSheinCandidateRecord`.

- [ ] Add failing test showing two active products with the same SDS supplier code use the same grouped effective cost when one has a manual cost.
- [ ] Implement a small candidate refresh helper that builds group costs from SDS supplier codes.
- [ ] Apply the helper before building candidate records.
- [ ] Run `GOWORK=off go test ./internal/listingkit/sheinsync -run TestRefreshCandidates -count=1`.

### Task 2: Candidate Profit Snapshot

**Files:**
- Modify: `internal/listingkit/sheinsync/candidate_service.go`
- Modify: `internal/listingkit/sheinsync/candidate_service_test.go`

**Interfaces:**
- Consumes: candidate `EffectiveCostPrice` and JSON `PriceSnapshot`.
- Produces: `CalculatedProfitRate` on candidate records.

- [ ] Add failing test showing `CalculatedProfitRate` is calculated from grouped cost and sale price.
- [ ] Implement candidate profit calculation with existing price snapshot format.
- [ ] Include calculated profit in candidate version hashing.
- [ ] Run `GOWORK=off go test ./internal/listingkit/sheinsync -run TestRefreshCandidates -count=1`.

### Task 3: Enrollment Cost Preservation

**Files:**
- Modify: `internal/listingkit/sheinsync/enrollment_service_test.go`
- Modify: `internal/listingkit/sheinsync/activity_adapter.go` if needed

**Interfaces:**
- Consumes: candidate snapshot `EffectiveCostPrice`.
- Produces: enrollment payload uses candidate snapshot cost consistently.

- [ ] Add or extend a test proving enrollment payload receives candidate effective cost and calculated profit.
- [ ] Keep adapter behavior compatible with existing promotion bridge.
- [ ] Run `GOWORK=off go test ./internal/listingkit/sheinsync -count=1`.

### Task 4: Verification

**Files:**
- No production edits expected.

- [ ] Run `GOWORK=off go test ./internal/listingkit/sheinsync -count=1`.
- [ ] Run `GOWORK=off go test ./internal/listingkit/... -count=1` if targeted tests pass.
- [ ] Review `git diff --stat` and final status.
