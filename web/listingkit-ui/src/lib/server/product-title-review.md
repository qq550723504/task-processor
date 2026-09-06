# Product title review browser boundary — #344

Authority: current Issues #344/#343/#345, N1-B comment 5560350419,
Product Review #333 contract, root AGENTS and Issue-driven development V1.
Admission: IMPLEMENTATION_READY, round 1 read-only boundary_review. No BLOCKER.
Unicode, trusted origin configuration, deadline/forwarded state, safe error
envelopes and #343 DTO alignment are IMPLEMENTATION_TEST obligations.
Go wire DTO: #343 current body, “N1-A 冻结的 planned Go HTTP 合同（2026-09-07）”.
Planned is not implemented integration evidence; actual Go HEAD still pending.

## Ownership and scope

Only collection GET, detail GET, decision POST and independent Apply POST under
`/api/product/text-proposals`. No collection POST or generic proxy. TS schema
and client live in `lib/api/product-title-review{,-client}.ts`; dedicated route,
proxy and request validation live in `lib/server/product-title-review*.ts`.
Product Review owns state, authorization, operation fingerprints and receipts;
Catalog owns publication in the existing shared PostgreSQL transaction. BFF has
no durable storage, automatic retry, operation key generation or state machine.
No UI changes. A separate test-only Go fixture and Node launcher own this task's
cross-process acceptance; existing read-only Listing fixture remains read-only.
Estimate: 16–24 scoped files, under 1,500 production lines. Reassess if exceeded.

## Invariants and trust

Use existing serverAuth and server token helper, selected effective-org cookie,
and X-Expected-Organization-ID equality. Browser token/actor/role/org-authority
headers are discarded. Rebuild only Accept, server Authorization,
X-Requested-Organization-ID, and for writes Content-Type/Idempotency-Key.
Go middleware reauthorizes every request: CachedRead on GET, LiveWrite on POST,
including receipt replay. Operator owner and selected-org admin rules remain Go's.
403 organization denial/revocation clears selection cookie using existing policy.

PRODUCT_REVIEW_API_ORIGIN is a server-only exact http(s) origin, without userinfo,
path/query/hash or default. No redirect following. Write Origin must equal the
existing configured public application origin; require same-origin Fetch Metadata
when present, application/json and expected-org header. Missing Origin fails.
Never derive trust from request Host, Forwarded or X-Forwarded-*.
Existing auth endpoint CSRF does not protect business POST; the dedicated boundary
must enforce and test this independently. No CORS permission or production bypass.

Path/method allowlist, canonical UUID, strict bounded query, actual body bytes
<=32KiB, duplicate/unknown fields and invalid Unicode rejected. Title <=4096 UTF-8
bytes without trimming. Reuse jsonc-parser strict JSON reader. Exact canonical
decimal versions remain strings in TS; validated expected revision is emitted as
the exact Go integer token without Number conversion. Go owns revision semantics.
Idempotency-Key supplied by UI is validated and forwarded unchanged.
Collection and all detail/decision/apply success responses <=128KiB, schema_version
1 and coverage product-title-proposals-only. Arrays are never null; absent receipt
is omitted. Exact fields and states follow #343, including snake_case quality.
Browser operation keys are single visible ASCII 1..128 bytes without comma (UUID
recommended), forwarded unchanged. This subset avoids Headers duplicate joining
and Unicode header encoding ambiguity; the original Go key contract is unchanged.

## Deadline and ambiguous writes

One 15s total deadline starts before authentication and covers body, HTTP and
response parsing, leaving space around Go's existing 10s budget. Link caller abort.
Keep a request-local forwarded flag shared with the route deadline response.
If authentication completes after timeout, check abort before any Go dispatch.
No second independent full deadline in proxy. Abort stops waiting, not a promise
that Go/PostgreSQL rolled back. Auth.js internal refresh remains its existing owner.

Before dispatch, failure is not_sent. Known safe Go validation/authorization/
conflict errors are rejected. After dispatch, network loss, timeout/cancellation,
invalid/oversize/truncated response, redirect and ambiguous Go 5xx are unknown:
return safe RESULT_UNVERIFIED communication envelope, never infer uncommitted.
Client additionally maps browser-to-BFF transport loss after fetch dispatch to
unknown, including abort. Preserve no raw upstream text. Never retry any POST.
Success returns validated Go DTO unchanged; accept does not send Apply.
Explicit user verification may replay the original org/actor/proposal/revision/
action/key/payload after authorization. A read lacking receipt proves no negative.
No cross-org/user automatic replay, browser ledger, queue, outbox or recovery owner.

## Verification and accepted scope

TDD: strict input/CSRF/header rebuilding; exact numeric strings; response bounds;
authorization failures/cookie clearing; one deadline through delayed auth/body;
cancel before/after forwarding; corrupt/lost response and POST count exactly one.
Check sibling decision/apply and list/detail success and failure paths together.

Actual Next BFF -> real Go application/middleware/Review/UoW/Catalog -> task-only
PostgreSQL. Real Sourcing/Catalog Publisher and Proposer/POST create successes.
Only external identity/session issuance, grants and CandidateGenerator are controlled
substitutes. Test-only fault transport can drop a response after real commit.
Prove collection, accept/edit/reject, independent Apply, stale/conflicts/replay,
reconstruction/receipt and unchanged non-title Product and Listing facts.
Separate unit substitutes, real isolated integration, NOT_RUN and skipped tests.
No paid models, real IAM, shared database, deployment or real accounts. Same-user
review and CachedRead are approved #333 risks, not new hardening blockers.
Independent final full-boundary review, applicable CI and UI consumption handoff
are required for completion. No merge/Issue close authorization.
