# 1688 Local Agent POC Design

## Goal

Validate a Windows command-line agent that performs a public 1688 product
crawl in the user's local browser environment and returns normalized source
input to the server. The server remains the authority for identity, task
state, source lineage, and every ListingKit action.

## Context

The server-side public crawl failed once with the generic error
`source_public_unavailable`. A direct local run of the same
`Alibaba1688Processor` against the same offer URL successfully initialized
Playwright, launched Chrome, navigated the page, and extracted the product.
That demonstrates an execution-environment difference, not a required 1688
account or a target-store problem.

The existing public-source path already treats `source_account_id=0` as a
clean, public crawl. `listing_store` is a target-marketplace concern and must
not be sent to, or selected by, the local agent.

## Scope

The POC provides one public 1688 source-execution loop:

1. An authenticated user creates a local-agent job for one validated 1688
   URL.
2. An authenticated Windows CLI agent claims one job for its tenant.
3. The agent runs the existing public 1688 processor in a clean local browser
   context.
4. The agent submits either a narrow 1688 product snapshot or a sanitized
   failure category.
5. The server validates the submission and reconstructs the canonical
   `SourceEnvelope` from the accepted snapshot.

The POC intentionally excludes ListingKit task creation, draft creation,
publishing, image uploading, background installation, auto-update, tray UI,
account-bound 1688 profiles, and long-lived agent registration.

## Chosen Architecture

Use pull-based Windows CLI execution. The CLI is started explicitly by the
user and polls for one job. The server never initiates a connection to the
client and does not store 1688 cookies, passwords, proxy credentials, or
browser profiles.

The POC introduces three isolated components:

- `internal/localagent`: server-side job lifecycle, tenant scoping, lease,
  result validation, and canonical source-envelope reconstruction.
- `internal/localagent/httpapi`: authenticated HTTP routes. It adapts request
  JSON to the application service and does not contain crawler code.
- `cmd/1688-local-agent`: a Windows CLI that performs device authorization,
  claims one job, runs the existing `internal/integration/crawler/a1688`
  adapter, and submits the narrow result.

The agent uses the existing `Alibaba1688Processor` through the integration
adapter. It does not import ListingKit or publishing packages. The server
uses the existing product-sourcing constructor to derive the `SourceEnvelope`;
it does not trust a client-supplied source identity or source key.

## Protocol

All routes are protected by the existing ZITADEL middleware and use its
trusted tenant context. No caller-controlled tenant identifier is accepted.

### Create a job

`POST /api/v1/local-agent/1688-jobs`

Request:

```json
{"url":"https://detail.1688.com/offer/1052008074197.html"}
```

The server normalizes and validates the URL, records a pending job for the
authenticated tenant, and returns `job_id` and `expires_at`. The job has a
twenty-minute lifetime. The POC stores jobs in process memory; restarting the
API discards pending jobs. Durable persistence is explicitly a follow-up
after the client execution contract is proven.

### Claim a job

`POST /api/v1/local-agent/1688-jobs/claim`

The server returns either `204 No Content` or exactly one tenant-owned pending
job:

```json
{
  "job_id":"...",
  "execution_token":"opaque-one-time-value",
  "url":"https://detail.1688.com/offer/1052008074197.html",
  "expires_at":"2026-08-21T00:00:00Z"
}
```

Claiming atomically changes the job to `claimed` and gives it a 15-minute
lease. The execution token is opaque, is never logged, and is valid only for
that job and lease. A job is deleted after its third expired claim lease;
earlier expired claims return to `pending` while the job itself is still valid.

### Submit a result

`POST /api/v1/local-agent/1688-jobs/{job_id}/result`

The request includes the execution token and exactly one terminal payload:

```json
{
  "execution_token":"opaque-one-time-value",
  "product_snapshot": {"id":"...","title":"..."}
}
```

or

```json
{
  "execution_token":"opaque-one-time-value",
  "failure":{"kind":"challenge","message":"sanitized diagnostic"}
}
```

The server accepts a submission only when the caller tenant owns the job, the
job is claimed, its lease is unexpired, and the execution token matches. It
validates that the result URL remains a normalized 1688 offer URL, limits the
encoded product snapshot to 1 MiB and a failure diagnostic to 512 UTF-8 bytes,
then reconstructs an
`Alibaba1688SourceEnvelope`. Duplicate or late submissions are rejected;
they never overwrite an accepted result.

## Authentication and Secret Handling

The CLI performs OAuth 2.0 Device Authorization against a user-supplied
issuer, client ID, and project ID. It follows the existing
`scripts/lib/listingkit-device-auth.ps1` safety behavior:

- HTTPS issuer and API URLs, except literal loopback test URLs.
- Discovery, device, token, and verification endpoints must be same-origin
  with the issuer.
- No redirects for OAuth requests.
- No `offline_access`, refresh token, token-file persistence, or token output.
- Bounded polling, provider `slow_down` handling, and a terminal deadline.

The access token is process memory only and is sent only as a bearer token to
the configured API origin. The agent never accepts a cookie, password, proxy,
browser profile path, `source_account_id`, or `listing_store` value from a
job.

## Failure Model and Observability

The client maps processor failures into one of `browser`, `navigation`,
`challenge`, `extraction`, or `unknown`. It sends a short sanitized diagnostic
without URLs containing credentials, HTML, cookies, headers, proxy values, or
browser-profile paths.

The server records the category in job state and logs the job ID, tenant
scope, state transition, and category. The public source error is therefore
observable without exposing source-account or browser secrets.

## Correctness and Security Invariants

- A job and result are visible only to the authenticated tenant that created
  the job.
- A claimed job has one active executor and one terminal submission.
- The agent can crawl only a canonical 1688 offer URL supplied by the server.
- The server, not the agent, computes source identity, source key, warnings,
  catalog facts, and asset facts.
- Target marketplace choices remain outside this POC.
- Failed agent runs cannot create a ListingKit task, draft, or publication.

## Verification

Automated tests cover URL validation, tenant isolation, one-job atomic claim,
claim expiry, execution-token rejection, duplicate-result rejection, result
size limits, source-envelope reconstruction, and sanitized failure handling.

The local acceptance sequence starts the API on loopback, uses a test OIDC
server, creates a job, runs the CLI with the existing local Chrome, and
asserts a completed job with a reconstructed source envelope. It must not
call ListingKit draft or publish endpoints.

## Deferred Work

Durable job storage, multi-agent scheduling, account-assisted crawling,
automatic binary updates, image download/upload, an interactive desktop UI,
and a bridge from a completed source envelope into a ListingKit task are
separate, later decisions.
