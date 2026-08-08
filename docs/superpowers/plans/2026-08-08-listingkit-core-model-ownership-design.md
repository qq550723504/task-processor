# ListingKit Core Model Ownership Design

## Problem

`internal/listingkit/model.go` and `internal/listingkit/core/model.go` independently define the same 13 task-lifecycle errors and the same `TaskStatus` type/constants. Equal error text does not give Go sentinel errors equal identity, so a producer returning the core sentinel can fail a consumer's `errors.Is` check against the root sentinel. The two named status types also represent the same domain concept without sharing a type identity.

## Decision

`internal/listingkit/core` is the only owner of the shared task-lifecycle errors and status type. The root `listingkit` package will not expose compatibility aliases. Every repository call site will import `internal/listingkit/core` and refer to `core.Err...`, `core.TaskStatus`, and `core.TaskStatus...` directly.

This is an intentional internal API break. The repository is migrated atomically so no checked-in revision depends on removed root symbols.

## Ownership Boundary

Core owns:

- `ErrTaskNotFound`
- `ErrTaskNotPending`
- `ErrTaskNotRecoverable`
- `ErrTaskRecoveryUnavailable`
- `ErrTaskRequeueUnavailable`
- `ErrTaskRequeueInvalidRequest`
- `ErrGenerationTaskNotFound`
- `ErrGenerationTaskNotRetryable`
- `ErrGenerationActionNotFound`
- `ErrChildTaskRetryInvalidRequest`
- `ErrChildTaskNotFound`
- `ErrChildTaskNotRetryable`
- `ErrChildTaskRetryConflict`
- `ErrSubmitInProgress`
- `TaskStatus` and its six constants

The root package continues to own errors that are specific to its higher-level submission, readiness, SHEIN cache, and category-search behavior. Those errors are not moved.

## Migration Shape

Root models change their field and function signature types from `TaskStatus` to `core.TaskStatus`. Root implementation and tests qualify every shared constant and sentinel through `core`. Child packages replace only references to the removed `listingkit` symbols; their other `listingkit` model and service references stay unchanged.

The migration uses `gofmt -r` for exact AST-aware symbol rewrites and `gopls imports` for import maintenance. No custom source rewriter is introduced.

## Regression Protection

An AST-based test in the core package parses the root package's non-test Go files and fails if any forbidden shared declaration reappears. This protects ownership rather than only checking today's file layout.

Verification includes:

- observing the ownership test fail before production edits;
- building all production packages after root symbols are removed;
- testing core, root ListingKit, API, store, Temporal, HTTP application, compatibility, product handoff, and enrichment packages;
- scanning for remaining `listingkit.<removed symbol>` references;
- `go vet`, `git diff --check`, and `go test ./...`.

## Risks and Controls

- Large mechanical diff: use exact symbol manifests, format every touched file, and inspect non-mechanical changes separately.
- Source-text boundary tests: update their expected qualified signatures together with the production signatures they guard.
- Accidental migration of unrelated domain statuses: root-directory rewrites operate only on the known ListingKit files; external rewrites require the `listingkit.` qualifier.
- Import collisions: use the package's declared name `core`, verify no touched file declares a conflicting local identifier, and let `gopls imports` remove or add imports.

## Non-goals

- Renaming the core package.
- Moving root-only errors into core.
- Consolidating similarly named task statuses in Amazon, product enrichment, product image, or other domains.
- Changing serialized status values, database column types, API response shapes, or runtime behavior.
