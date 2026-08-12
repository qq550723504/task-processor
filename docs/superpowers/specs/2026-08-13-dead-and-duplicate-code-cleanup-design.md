# Dead and Duplicate Code Cleanup Design

## Goal

Remove all code that can be proven unused and consolidate all code that can be proven to represent the same responsibility, while preserving supported and intentionally deferred capabilities, runtime behavior, public contracts, build-tag variants, test intent, and the repository's established ownership boundaries.

## Context

This repository is a large Go modular monolith with a Next.js/TypeScript frontend and maintained support code under `scripts`, `tools`, and `hack`. The current checkout contains 4,119 Go files, 437 TypeScript files, 348 TSX files, and 5 MJS files after excluding worktrees, dependency directories, generated build output, local runtime state, and temporary modules.

The initial read-only scan established that cleanup cannot be driven by a single detector:

- `golang.org/x/tools/cmd/deadcode@v0.48.0`, rooted at the four maintained Go commands and including tests, reported 1,618 unreachable-function candidates on the current Windows configuration.
- `knip@6.32.2` reported 5 unused frontend files, 1 unused dependency, 1 unused development dependency, 58 unused exports, and 33 unused exported types.
- `jscpd@5.0.14`, using a 12-line and 70-token minimum, reported many clone candidates across active, compatibility, marketplace, store, HTTP-module, and test-support code.

These are candidates, not deletion instructions. The Go report includes clusters in intentionally deferred Amazon code and runtime assembly code. The Knip report includes generated contract types and type-test files that require configuration-aware handling. The clone report includes both genuine same-owner duplication and independent business policies that happen to have similar syntax.

The checkout also contains an unrelated untracked plan file. Cleanup work must preserve it and any later unrelated changes, and every commit must stage only the paths belonging to its cleanup batch.

## Scope

The cleanup covers all tracked code-bearing areas in phases:

1. maintained Go production commands and their reachable packages;
2. `web/listingkit-ui` production TypeScript and TSX;
3. Go and frontend tests and test support;
4. maintained code under `scripts`, `tools`, and `hack`;
5. source-code dependencies, generated-code inputs, and code-generation configuration;
6. executable configuration or templates when they contain obsolete registrations or duplicate logic.

Documentation and Kubernetes overlays are evidence for ownership and supported behavior, not token-clone cleanup targets. Obsolete documentation or deployment assets discovered while removing a retired entrypoint must be handled in the same retirement batch, but intentional overlay repetition is not duplicate source code.

Generated files are never hand-edited to satisfy a detector. A generated symbol is removed by changing its authoritative schema or generator inputs and regenerating the output. If the complete generated surface is intentionally published as a contract, unused generated exports are configured as such rather than deleted individually.

## Definitions

### Dead code

Code is dead only when all relevant evidence agrees that it has no supported execution, contract, generation, test, operational, or intentionally deferred role.

A candidate is removable when:

- it is unreachable from every maintained runtime entrypoint that could own it;
- it is not reached only under another supported `GOOS`, `GOARCH`, build tag, or command configuration;
- it is not registered dynamically through reflection, `init`, HTTP route composition, Temporal/RabbitMQ registration, dependency injection, configuration names, templates, or generated code;
- it is not an interface method, serialization field, migration hook, command flag, environment/configuration key, or compatibility contract required by a maintained caller;
- tests, documentation, deployment assets, and repository history do not identify it as a supported or intentionally deferred capability;
- removing it does not expose a missing production registration that should be repaired instead.

Code used only by tests is not automatically production code. It must either remain clearly test-owned, move into a test file/support package, or be removed with the tests that require it. A test is dead only when it no longer validates a supported invariant, regression, or contract.

An entire dormant subsystem is not removed as a side effect of leaf cleanup. It receives an explicit retirement decision based on current product-authority documents and runtime ownership. If it remains intentionally deferred, it is classified as retained dormant capability rather than falsely claimed as cleaned dead code.

### Duplicate code

Detector similarity is duplicate code only when the implementations:

- enforce the same invariant or perform the same transformation;
- have the same business owner and reason to change;
- can share one existing or clearly owned implementation without reversing dependency direction, weakening types, or creating a generic utility bucket;
- can be covered by focused characterization tests before consolidation.

Similar HTTP wiring, repository CRUD, enum handling, validation tails, or marketplace policies with different owners are not consolidated merely to reduce clone counts. Cross-domain consolidation is allowed only when an existing neutral package already owns the shared concept or when the change introduces a narrow, named contract at the correct boundary. The cleanup must not create a new parallel subsystem or a second owner for business state.

## Chosen approach

Use an evidence matrix and small, independently verifiable cleanup batches.

The alternatives were rejected as follows:

- A big-bang automated deletion cannot distinguish dormant supported code, dynamic registrations, generated contracts, and detector false positives.
- A domain-by-domain manual review without repository-wide tools would miss unused dependency chains, orphan files, and cross-file clones.

The evidence-first approach keeps repository-wide coverage while requiring human ownership and runtime evidence before each deletion or extraction.

## Analysis toolchain

Reuse maintained open-source analyzers instead of implementing custom parsers:

- Go reachability: `golang.org/x/tools/cmd/deadcode@v0.48.0`.
- Go correctness and unused constructs: the repository's configured Staticcheck, `go vet`, compiler, and focused `go test` runs.
- Frontend graph analysis: `knip@6.32.2`.
- Token clone detection: `jscpd@5.0.14`.
- Exact references and dynamic-registration evidence: `rg`, `go list`, existing import-boundary tests, and existing dependency-baseline scripts.

Versions and detector configuration must be pinned in repository-owned scripts or configuration so the final baseline is reproducible. Detector exclusions must be narrow and explained. They may exclude generated output, vendored dependencies, local state, worktrees, build output, and intentionally unsupported file formats; they must not hide production directories or broad issue classes.

The Go reachability scan runs at least these roots and configurations:

- `cmd/product-listing-api`;
- `cmd/listing-control-plane`;
- `cmd/shein-listing`;
- `cmd/temu-listing`;
- test roots enabled and disabled;
- Windows and Linux target configurations;
- any repository-supported build tags discovered from CI, Makefile, Dockerfiles, or source constraints.

The intersection identifies high-confidence dead candidates. Differences identify configuration-specific code that requires manual evidence rather than deletion.

Knip is configured with Next.js, Vitest, generated-client, type-test, and script entrypoints before acting on its report. Findings are processed top-down: unused files, then exports/types, then dependencies. Removing an orphan file may eliminate downstream exports and dependency usage, so counts are refreshed after every batch.

jscpd reports are triaged by ownership before line count. Exact or near-exact clones inside one package or one business capability are handled first. Cross-marketplace and cross-runtime matches are reviewed against stable boundary documents and are not promoted into generic helpers solely to satisfy the detector.

## Execution phases

### Phase 0: Reproducible baseline

Add pinned analyzer configuration and a read-only code-health script. Record candidate counts by category and package without committing generated bulk reports. Establish the current test/build baseline before deleting code. Classify baseline failures as pre-existing or cleanup-related; never use an unrelated failure to waive verification.

### Phase 1: High-confidence frontend cleanup

Review the five unused-file candidates and their route, test, dynamic import, and generation relationships. Remove proven orphan files, then remove proven unused exports and dependency declarations. Generated contract exports and type-test entrypoints are resolved through Knip configuration or authoritative generator inputs. Run focused tests after each component or API-client batch, then lint, typecheck, test, and build the complete frontend.

### Phase 2: High-confidence Go leaf cleanup

Start with unreachable leaf functions and files inside active packages, not dormant subsystem roots. For every candidate batch, inspect references, interfaces, registrations, build constraints, config keys, and history. Add or preserve characterization tests when behavior is consolidated; pure unreachable deletion does not require a synthetic test, but the owning package and all maintained command builds must remain green.

Process candidates by stable owner so a reviewer can accept or reject one cleanup independently. Do not combine marketplace-policy changes, package moves, or behavior fixes with mechanical deletion.

### Phase 3: Same-owner duplicate consolidation

Prioritize exact or near-exact copies that already have one obvious owner, including duplicate transformations, repeated success/failure tails, repository methods, route-module builders, and compatibility mirrors. Characterize observable behavior before extracting or promoting the canonical implementation. Delete old copies only after every caller uses the canonical owner.

If two clones differ in error semantics, ordering, tenant checks, retry behavior, persistence rules, or marketplace policy, keep them separate until those differences are explicitly modeled. A smaller line count is not success if it obscures ownership.

### Phase 4: Compatibility and dormant-subsystem decisions

Review clusters in historical compatibility packages, old runtime assembly, and deferred Amazon/TEMU capabilities as whole subgraphs. For each cluster, produce one of three evidence-backed outcomes:

- retire and delete the subsystem plus registrations, tests, dependencies, docs, and deployment references;
- reconnect a supported but accidentally unreachable entrypoint, treating the finding as a defect rather than dead code;
- retain an intentionally deferred capability with a named owner, support rationale, and detector exception limited to the exact entrypoints or files.

This phase prevents the goal from being falsely completed by leaving thousands of unexplained candidates or by deleting product commitments implicitly.

### Phase 5: Tests, scripts, tools, and executable configuration

Remove obsolete test helpers, superseded regression tests, retired scripts, unreachable tool commands, duplicated PowerShell/shell logic, and obsolete executable registrations. Preserve tests that protect current invariants even when production call graphs do not reach their helpers. Shared script logic is centralized only when platform, shell, and failure semantics match.

### Phase 6: Dependency and prevention closeout

Run `go mod tidy`, package-manager dependency checks, API generation checks, and repository structure tests after code removal. Keep lightweight pinned analyzer commands available for future cleanup. New gates should fail only on newly introduced high-confidence findings or on an explicitly ratcheted baseline, not on unexplained legacy output.

## Verification strategy

Each cleanup batch must include:

1. a before/after candidate list for the owned paths;
2. exact-reference and dynamic-registration checks;
3. focused package or frontend tests;
4. relevant boundary and repository-structure tests;
5. build verification for affected maintained commands;
6. a scoped Git diff proving no unrelated file was changed or staged.

Final verification must include, on the exact final commit:

```powershell
go test ./... -count=1
go test -race ./internal/app/runtime/listingcontrol -run TestControlPlaneService -count=1
go test -race ./internal/listingadmin -run "TestConcurrentClaimForDispatchOnlyOneWorkerWins|TestConcurrentRollbackDispatchOnlyOriginalQueuedClaimIsRestoredOnce|TestConcurrentRecoveryOnlyUpdatesStillEligibleRowsOnce" -count=1
make build-all

Set-Location web/listingkit-ui
npm.cmd ci
npm.cmd run lint
npm.cmd run typecheck
npm.cmd test
npm.cmd run build
```

On Windows, if GNU Make is unavailable, the four maintained commands are built directly with the equivalent `go build` invocations. Race tests must run on a supported platform; an unsupported local race environment is recorded and verified in CI rather than silently skipped.

The final analyzer audit must rerun all pinned configurations. Completion requires:

- no unexplained high-confidence dead-code candidate in the agreed code scope;
- no unused frontend file or dependency outside generated-contract or explicitly documented entrypoint rules;
- every remaining unreachable cluster classified as supported configuration-specific code, intentionally deferred capability, generated contract, or verified detector limitation;
- every remaining clone above the configured threshold classified by owner, with genuine same-owner duplication removed;
- no unrelated user work modified, deleted, or staged;
- all required tests and builds passing, or any external/environmental gate reported as unverified without claiming the goal complete.

## Deliverables

The implementation produces:

- pinned, reusable analyzer configuration and invocation scripts;
- small cleanup commits grouped by stable owner;
- a concise decision ledger for retained detector findings and dormant subsystems;
- refreshed dependency manifests and generated output only when their authoritative inputs changed;
- final before/after counts, test/build evidence, and an explicit list of any remaining external verification gates.

No branch publication, pull request, merge, deployment, provider activation, database mutation, or production operation is part of this design without separate authorization.
