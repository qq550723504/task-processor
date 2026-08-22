# Local-agent 1688 contract hardening

## Context

PR #170 introduced a local 1688 crawler agent that submits a product snapshot
to the server.  Review feedback after merge exposed four symptoms:

- credential-bearing asset URLs can enter source handoff facts;
- `price_range_count` is client supplied even though the submitted contract has
  no price ranges from which to derive it;
- a retry with the original execution token after a terminal transition is
  reported as an authorization failure instead of a terminal job;
- cancellation reaches the runner but not the legacy browser crawl, so a
  canceled job can keep navigating until its own timeout.

These are one boundary problem rather than four independent defects: the
external-agent protocol has no single admission policy and its lifecycle
contract is only partially represented at its adapters.

## Decision

Make the local-agent submission contract deliberately narrow and fail closed.
This is a breaking cleanup: no compatibility behavior is retained for the
unused pre-release protocol.

### 1. Admit only validated source facts

Keep `sourcing.Alibaba1688ProductSnapshot` as the product-facing data contract,
but make `localagent.Service.SubmitSuccess` its only server admission point.
Its validation will retain the existing `ProductChecker` semantic checks and
will consolidate URL parsing into one policy helper used by every URL-bearing
snapshot field.

The helper accepts only absolute HTTP(S) URLs with a hostname and no user-info.
It has two explicit policies:

- media assets may retain query strings because CDN signed URLs depend on them;
- supplier shop URLs require HTTPS and reject user-info, query strings, and
  fragments because they are persisted as reusable supplier facts.

Validation errors name a field/index only; they never interpolate the rejected
URL, so credentials cannot be reflected into an API response or log entry.

All of main image, gallery, video URL/cover, product-detail image, variant
image, package image, and supplier shop URL use this policy before envelope
construction.  `SourceEnvelope` remains a pure transformation and may assume
an admitted snapshot.

### 2. Remove unverifiable derived data

Delete `PriceRangeCount` from the source snapshot, local-agent HTTP request,
client payload, legacy snapshot adapter, and source-envelope facts.  The
protocol transfers no price-range collection, so a count is neither source data
nor a verifiable fact.  A future requirement for price ranges must add the
actual range collection and derive a count server-side; it must not re-add a
client asserted count.

### 3. Make terminal retries distinguishable from unauthorized submissions

Each claimed record keeps the active execution token only while claimed.  On a
successful or failed terminal transition, replace it with a SHA-256 digest of
that token; never retain the raw token in the terminal record.

`claimRecordLocked` first verifies record ownership, then handles terminal
records.  A constant-time match of the presented token digest returns
`ErrTerminalJob`; a missing or mismatched token returns `ErrInvalidClaim`.
For active records it retains the present lease and token checks.  Reclaiming a
lease clears any terminal digest and installs a new active token.

This preserves the HTTP contract: a genuine duplicate gets `409
job_not_active`, whereas another caller cannot use terminal state as an
authorization oracle and receives `403 claim_denied`.

### 4. Propagate cancellation to the browser owner

Replace the legacy `Process(url)` integration seam with
`Process(context.Context, url)`.  Propagate the context through the integration
adapter, the 1688 processor, single-processor methods, and worker call sites.

Once a browser page is created, register `context.AfterFunc` with a
`sync.Once`-guarded cleanup function.  Cancellation closes that job's browser
manager, unblocking Playwright navigation, extraction, and manual waits.  Each
stage also checks `ctx.Err()` and returns it preferentially after an interrupted
browser operation.  Normal completion stops the callback and closes the
browser exactly once.

The runner already sends failures through a short-lived context derived with
`context.WithoutCancel`; therefore a canceled crawl terminalizes as the
sanitized navigation failure rather than leaving its claim until lease expiry.

## Data flow

```text
local crawler
  -> typed HTTP payload (no derived count)
  -> localagent Service admission
       -> URL and semantic validation
       -> SourceEnvelope transformation
       -> terminal state transition
  -> bounded terminal response

runner context cancellation
  -> integration Processor
  -> legacy processor / single processor
  -> context.AfterFunc closes job browser
  -> context error
  -> bounded failure submission
```

## Rejected alternatives

### Continue adding per-field checks

This would address the current four comments but leaves future snapshot fields
able to bypass a common URL policy or be copied into canonical facts without a
derivation rule.

### Keep and cap `price_range_count`

A numeric cap does not make the value true.  Because the contract has no
price-range items, omission is the only honest representation.

### Return success for every duplicate submission

Returning a prior terminal response would require retaining a larger completed
payload and changes the current error contract.  Recognizing the original
token and returning the existing `ErrTerminalJob` is sufficient, avoids raw
token retention, and maintains authorization separation.

### Cancel only in the runner

The runner cannot interrupt the underlying blocking Playwright call.  The
browser owner must receive the context and close the browser it created.

## Test and review gates

- Table-driven service and product-checker tests cover credential-bearing URLs
  at every asset and supplier URL field, plus safe signed media URLs.
- HTTP/client payload tests prove `price_range_count` is absent, and source
  envelope tests prove it cannot appear in facts.
- Service and HTTP tests prove the original terminal token gets `409
  job_not_active`, while a wrong token gets `403 claim_denied`.
- Integration and single-processor tests prove context reaches the source and
  cancellation invokes browser cleanup once and returns promptly.
- Focused tests run with `-race`; the affected Go packages, root `go test
  ./...`, local-agent build, acceptance Pester suite, and `git diff --check`
  are required before publishing a Draft follow-up PR.

## Scope boundaries

This change does not start a live 1688 crawl, change tenant or account
selection, deploy anything, or alter ListingKit workflow ownership.  It only
hardens the already merged local-agent protocol and its browser cancellation
path.
