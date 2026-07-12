# SDS POD Test Hardening Design

## Goal

Strengthen the two regression guards identified during the SDS POD canonical metadata refactor without changing production behavior, public contracts, or package ownership.

## Scope

This slice changes test code only.

1. Add a behavioral regression test for `service.refreshSheinDerivedState`.
2. Replace the SDS metadata boundary test's fragile string checks with AST-backed checks across all ListingKit production Go files.

## Revision Recompute Regression Test

The refresh path now delegates SDS style application to
`sdspod.ApplyCanonical`. The test will construct a task with a canonical
product containing multiple variants and an SDS StyleName. It will invoke the
existing refresh entrypoint with a request that reaches the normal refresh
path. It will assert every canonical variant has:

- attribute key `ai_style`;
- the trimmed configured style value;
- the exact existing derived trace: `SDS studio AI style dimension`, confidence
  `0.94`, `IsInferred: false`, and `NeedsReview: false`.

The test uses the real refresh path and canonical mutation rather than a mock.
It does not assert unrelated SHEIN derived state.

## AST Boundary Guard

The existing `TestWorkflowStudioSDSMetadataSupportBoundary` will retain its
current file-ownership assertions, but SDS delegation assertions will be
structural:

- scan every non-test Go file in `internal/listingkit`;
- fail if any function declaration named `applyStudioStyleDimension` exists;
- parse `sds_canonical_metadata.go` and require the exact sdspod import path;
- require a real selector call expression `sdspod.ApplyCanonical`, not a text
  occurrence in a comment or string.

The AST helpers will live in test code and have narrow, reusable names. They
will parse source with Go's standard `go/parser` and inspect declarations,
imports, and call expressions with `go/ast`.

## Non-Goals

- No production Go changes.
- No modification to `sdspod.ApplyCanonical`, DTOs, JSON contracts, images,
  workflow ordering, remote sync, or SHEIN payload behavior.
- No generic test framework or reflection-based scanner.
- No cleanup of unrelated legacy string assertions.

## Testing and Verification

Follow TDD for each guard: write it against the absent helper/call condition,
observe the expected failure, then add only the test helper or fixture needed
to make the current implementation pass. Verify with:

~~~powershell
go test ./internal/listingkit -run "TestRefreshSheinDerivedState|TestWorkflowStudioSDSMetadataSupportBoundary" -count=1
go test ./internal/listingkit/... -count=1
go vet ./internal/listingkit/...
~~~

The main worktree's existing `go.work.sum` modification remains out of scope.

## Success Criteria

- Removing the style-only delegation from revision refresh causes its direct
  behavior test to fail.
- A comment or string containing `sdspod.ApplyCanonical` does not satisfy the
  boundary guard.
- Reintroducing `applyStudioStyleDimension` in any ListingKit production file
  causes the boundary guard to fail.
- Current production behavior and all public contracts remain unchanged.
