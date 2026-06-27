# Current Refactoring Status

> Status: active current-state document.
>
> Last reviewed: 2026-06-27.
>
> Scope: refactoring closeout, growth sequencing, and the current Now / Next / Later direction for Task Processor / ListingKit.

## 1. Current position

The project is past the early broad-splitting phase.

The current posture is:

```text
Close out the new runtime and boundary ownership first;
expand product sources second;
defer full new sales-platform workbenches until the SHEIN template is stable.
```

This document is intentionally short. It points to the active execution and checkpoint documents instead of repeating every historical slice.

Use this file together with:

- `docs/refactoring/next-phase-plan.md`
- `docs/refactoring/listingkit-boundary-checkpoint.md`
- `docs/refactoring/decisions/2026-06-26-next-growth-sequence.md`

## 2. Now

The current active work should focus on closeout and stabilization, not broad feature expansion.

### 2.1 Refactoring closeout

Required next work:

1. Keep the listing runtime boundary free of `ManagementClient`; the retired `internal/infra/clients/management` package no longer exists as a Go package, and boundary tests keep the old import paths from being reintroduced.
2. Keep bootstrap shared resources on the `internal/listingruntime/local` provider/runtime path; that package now owns the local runtime implementation used by bootstrap. HTTP task-RPC/login and processor-base seams use explicit ports or retired/unavailable behavior instead of reviving a broad Management Client.
3. Keep `internal/listingkit` as orchestration, compatibility, DTO adaptation, persistence ordering, and API shell glue.
4. Do not continue splitting files just because a helper can move.
5. Keep every new migration tied to an explicit ownership reduction.

### 2.2 Control Plane and runtime validation

The backend Go Listing Control Plane hardening path has production closeout evidence, but the current closeout still needs stable operational proof around the broader runtime.

Required next work:

1. Record the latest `master` validation for:
   - `go test ./... -count=1`
   - listing control-plane race tests
   - listingadmin dispatch / rollback / recovery race tests
   - `go build ./cmd/listing-control-plane`
   - `go build ./cmd/shein-listing`
2. Keep the SHEIN listing browser startup path in the rollout smoke test checklist.
3. Publish or explicitly schedule the frontend admin dispatch-reason / dispatch-event visibility if the UI artifact deploys separately.
4. Keep dispatch event distribution, leader status, and `failed > 0` cycle reporting as observability follow-ups rather than introducing another scheduler variant.

### 2.3 Boundary checkpoint cleanup

Required next work:

1. Treat `listingkit-boundary-checkpoint.md` as the current ListingKit stop-line authority.
2. Keep long completed-slice lists as evidence, not as an invitation to continue helper shaving.
3. Prefer small target-domain policy seams with guard tests.
4. Update allowlists only with a temporary ownership explanation.

## 3. Next

After the closeout items are green or explicitly documented, the next growth direction should be product-source expansion.

### 3.1 Product-source expansion focus

Preferred target area:

```text
internal/product/sourcing
internal/catalog or the approved product/catalog target
internal/asset or the approved product/asset target
internal/integration/crawler/*
```

Allowed work:

1. Product source identity.
2. Source result normalization.
3. Canonical product facts.
4. Image and asset fact normalization.
5. Cost, price, and source-SDS identity mapping when it stays platform-neutral.
6. Thin crawler adapters that execute raw source collection without owning marketplace publishing rules.

Stop lines:

1. Product-source code must not directly assemble SHEIN publish payloads.
2. Crawler packages must not depend on ListingKit root, marketplace publishing, or workspace packages.
3. New source logic must not add `if source == ...` policy branches to root `internal/listingkit` unless it is temporary API-shell adaptation with a recorded follow-up.

## 4. Later

Full new sales-platform expansion should wait until the SHEIN template is stable enough to copy without copying the old coupling.

Allowed now:

1. Platform capability inventory.
2. API / readiness contract design.
3. Mapping cost assessment.
4. Read-only package guards.
5. Platform 자료包 / payload preview exploration that does not introduce a second submission state machine.

Deferred for now:

1. Full TEMU / Amazon / Walmart workbench expansion.
2. New platform auto-publish runtime.
3. Another dispatch scheduler or watchdog owner.
4. Marketplace-specific rules added to root `internal/listingkit`.
5. A new submission state machine outside `internal/listing/submission` and marketplace-owned publishing packages.

## 5. Do not do now

Do not start work that does any of the following:

- renames broad package trees for directory consistency only;
- moves files without reducing ownership or dependency pressure;
- combines behavior changes with package movement;
- expands import boundary allowlists without a migration explanation;
- adds business rules to `internal/app/*` runtime assembly;
- adds SHEIN, TEMU, Amazon, or Walmart platform policy to root `internal/listingkit`;
- splits into microservices before package boundaries are stable;
- launches a full new sales-platform workbench before closeout and product-source normalization are stable.

## 6. Current execution checklist

Use this checklist before approving the next structural migration PR:

```text
[ ] The target package does not import internal/listingkit.
[ ] The moved logic is a stable rule or policy, not runtime wiring.
[ ] ListingKit keeps only compatibility, DTO adaptation, orchestration, or persistence callbacks.
[ ] The PR includes a focused test or an import-boundary guard.
[ ] Behavior changes are separated from file moves.
[ ] The change does not touch Temporal determinism unless explicitly reviewed.
[ ] The change does not add a second owner for dispatch, recovery, or submission state.
[ ] The latest validation result is recorded or the missing validation is called out.
```

## 7. Source of truth summary

Current order of authority:

1. `current-refactoring-status.md` for Now / Next / Later.
2. `next-phase-plan.md` for immediate execution details.
3. `listingkit-boundary-checkpoint.md` for ListingKit stop lines.
4. `project-wide-refactoring-plan.md` for long-term architecture direction.
5. `project-wide-execution-plan.md` and dated progress snapshots as historical references.
