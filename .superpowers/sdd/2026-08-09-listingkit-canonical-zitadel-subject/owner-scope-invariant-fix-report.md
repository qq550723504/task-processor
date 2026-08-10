# Owner-scope invariant fix report

## Status

The ListingKit and ListingAdmin owner scopes are now enabled by default before
HTTP bootstrap. The former production boolean setter has been removed. HTTP
bootstrap invokes the enable-only `EnableOwnerScope` API for both packages, so
it can re-enable a scope that a test temporarily disabled but cannot provide a
production disable path.

## TDD evidence

Added one behavioral regression test per package:

- `listingkit.TestOwnerScopeEnabledByDefault`
- `listingadmin.TestOwnerScopeEnabledByDefault`

The tests were run before the production change and both failed because the
zero-value `atomic.Bool` left the owner scope disabled. After the default-on
initialization and enable-only API were implemented, both tests passed. The
existing HTTP bootstrap test now explicitly verifies that temporarily disabled
test scope is re-enabled by bootstrap.

## Test-helper boundary

`SetOwnerScopeRequiredForTesting(bool)` remains explicitly test-named. It must
remain exported because external-package tests in `internal/listingkit/httpapi`
exercise both ListingKit and ListingAdmin bootstrap behavior, and ListingKit
store and studio-store tests import `internal/listingkit`. It is not wired to
YAML, environment variables, or any production configuration route. The only
production mutator is the enable-only `EnableOwnerScope` method.

## Fixture alignment

Store handler and store-statistics tests that seed rows intended to be visible
to a request now include the matching `owner_user_id`; this makes their fixture
behavior match the fixed production owner-scope invariant instead of relying on
the previous disabled default.

## Verification

Commands run successfully after the change:

```text
go test ./internal/listingkit ./internal/listingadmin ./internal/listingkit/httpapi ./internal/core/config -count=1
go vet ./internal/listingkit ./internal/listingadmin ./internal/listingkit/httpapi ./internal/core/config
git diff --check
```

An `rg` inventory confirmed there are no Go call sites of
`ConfigureOwnerScopeRequired` under `internal`.
