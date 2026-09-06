# Deterministic Validator Contract — Issue #34, slice A

## Content-bound diagnostics (Issue #318)

The v1 API and strict Snapshot checks remain unchanged. The opt-in SHEIN
`DiagnosticValidator.Validate(BoundRequest[[]byte])` uses rule version
`shein.offline_package.v2` and binding version `shein.persisted-input.go-json.v1`.
It accepts persisted JSON, not an in-memory Package with private resolver state.
Both versions call one shared evaluator; neither retries the other version.

`publishing/shein.DecodePersistedPackageStrict` creates the exclusive normalized
copy used for both SHA-256 identity and rule evaluation. Raw input, normalized
binding envelope, and encoded report each have a 2 MiB limit; JSON depth is 64.
Duplicate/unknown/case-mismatched fields, invalid UTF-8 or surrogate pairs,
numeric overflow/underflow, conflicting aliases and trailing values fail closed.
Strict field decoding reuses pinned `sigs.k8s.io/json`; the root Package custom
decoder is bypassed, with a test auditing reachable custom decoders. Numbers use
the current Go schema's numeric semantics, including ordinary binary64 rounding.
Private resolver maps cannot be supplied through persisted JSON.

Binding hashes Go JSON encoding of the ordered envelope `binding_version`,
`marketplace`, `site`, `action`, `rule_version`, `package`. Map keys are sorted;
array order is significant. Existing Package normalization/MarshalJSON defines
alias and nil/empty behavior; fixed vectors pin these and nested SDK fields.
This is a versioned Go encoding, not a cross-language JSON canonicalization API.
Future writers may use existing `json.Marshal(Package)` and must admit the bytes
through `DecodePersistedPackageStrict`; encoding alone is not admission. This
slice introduces no persistence writer/helper, schema or alternate encoder.

ReadAt and EvaluatedAt must be supplied and ordered. They do not enter the digest
and do not establish freshness. ExpectedDigest is an optional independent compare
against loaded content, not CAS, a permission proof or an automatic self-check.
External freshness must be explicitly `not_evaluated` with no evidence, or carry
bounded owner/policy identifiers, subject digest, observation and exclusive expiry
times. No current time or TTL is invented. Well-formed known adverse evidence
returns StaleInput even for partial coverage; partial valid evidence is rejected.
Evidence is a trusted caller assertion, not authenticated by this pure package.
Consumers requiring freshness must call RequireFreshness and cannot treat unknown
as valid. Live template, authorization, cookie, POD, human review, asset consent
and submission gates remain explicitly not evaluated.

The result is diagnostic_only with scoped offline_checks and separate action
policy, never a top-level submission permission. Every operational error returns
a zero result; business blockers remain successful diagnostic findings. The
caller owns loading, deadlines and cancellation before/after bounded synchronous
compute. No I/O, goroutines, clock, randomness, persistence or runtime wiring is
introduced. Organization reads and writer ownership remain Issue #319; #315 is
paused. v1 has no production HTTP/Tool callers to migrate in this slice.

This is an offline, compute-only preparation slice under the Marketplace owner.
Authority: Issue #34, `docs/architecture/project-target-architecture.md`, Product
Phase 3 and the Legacy Hard-Cut Policy. It does not complete Issue #34.

## Contract and invariants

- `Validator[T].Validate(Request[T])` accepts caller-owned immutable facts. The
  generic payload keeps platform DTOs out of the neutral contract. Adapters own
  their typed input and supported target/action/rule version, not Product facts.
- Each request pins an opaque snapshot revision and rule version. A supplied
  expected revision must match. Freshness uses supplied evaluation/observation/
  expiry timestamps, never the wall clock. Expiry is exclusive. Revisions are
  caller assertions, not hashes or authorization proofs; callers must bind them
  to the actual loaded snapshot. New facts or rules require new revisions.
- Results distinguish `ready`, `ready_with_warnings`, `blocked`. Business rule
  failures are structured findings with nil Go error. Malformed input,
  unsupported target/action/version, stale snapshots and inability to evaluate
  return a typed error and a zero result: never a successful empty report.
- Findings preserve rule key, code, category, paths, message and human guidance.
  Sort by stable content for deterministic output; do not merge distinct checks
  merely because they share a rule key (SHEIN has two `variants` checks).
- `Scope` explicitly names the evaluated rules. Ready means only that scope
  passed. `ReadinessBlockersAllowed` reports existing action policy separately
  and cannot change blocked into ready. Neither field authorizes submission.
- No network, model, database, file write, persistence, retry or state machine.
  No authorization/tenant boundary changes. The caller retains access checks and
  owns snapshot loading; no validator refresh or automatic repair.

## SHEIN adapter scope

`internal/marketplace/shein/validator` evaluates `shein.offline_package` using
existing workspace template/payload/review rules and publishing payload checks.
It clones the package using the existing owner clone before normalization.
No new dependencies on root ListingKit or compatibility owners are permitted.
`preview` is explicitly unsupported in v1: do not silently map it to publish.
`save_draft` and `publish` reuse the existing action policy without changing it.
Site overrides are explicitly unsupported in v1 (empty `Target.Site`): the
existing package rule groups do not establish per-site validation coverage.

Not evaluated: login/auth evidence, live template freshness acquisition, POD execution state,
ProductSnapshot/ApprovedAsset loading or approval provenance, current store
authorization, runtime submission gates. Snapshot freshness here covers the
supplied offline package only. An image URL passing an existing package rule is
not proof of ApprovedAsset consent. Future typed adapters can carry canonical
facts, but this slice does not invent a second Product/Asset model.

No UI, fixed pipeline, Tool (#134), Agent or submission entrypoint is switched in
this slice. Their complete reuse/cutover and end-to-end parity remain on #34.
No historical state migration or external compatibility exception is needed.

## Admission and evidence plan

Bounded pure computation: Marketplace contract/adapter plus existing publishing
calls, no new consistency boundaries, authorization or state machine. Under 30
files / 1,500 production lines. Runtime integration is split out to avoid
crossing Product, Listing, Tool and UI boundaries in one change. Shared docs and
tests owned by #311 remain untouched. Independent review checks this boundary.

TDD evidence: malformed/unsupported/stale errors, exact freshness boundaries,
required versus optional attributes, draft versus publish, missing image,
prepared payload failures, deterministic repeat/concurrent evaluation and input
immutability. Dedicated import guard forbids Agent/provider/legacy dependencies
in the neutral contract and new adapter. Final SHA/CI/review evidence lives in PR.
