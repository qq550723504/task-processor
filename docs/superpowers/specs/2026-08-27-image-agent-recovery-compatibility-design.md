# Image Agent Recovery and Compatibility Design

## 1. Status and decision

- Date: 2026-08-27
- Baseline: `702d76631`
- Status: design approved in chat; implementation has not started
- Scope: harden phase-one manual image generation only
- Durable artifact store: the configured S3-compatible object store, including COS-compatible endpoints
- Temporal rollout: frozen v2 contracts plus new v3 activity names on a dedicated v3 task queue

This design replaces the current collection of partially recoverable side effects with one versioned recovery protocol. Provider generation, durable staging, final publication, approval, and workflow replay each receive an explicit owner and compatibility boundary. The design does not claim that an arbitrary third-party API can execute physically exactly once. It guarantees one durable, observable business result when the destination supports deterministic keys or idempotency and blocks rather than guesses when a remote outcome cannot be reconciled.

## 2. Context and root cause

The current manual image workflow persists a `generated_complete` receipt after generation, but the receipt can contain Pod-local paths and unrestricted provider metadata. The worker uses an `emptyDir`, so a retry on another Pod can find a completed database phase whose bytes no longer exist. Publication also has no independent claim owner, lease, or fencing token, allowing concurrent retries to invoke the publisher at the same time.

Two Temporal compatibility boundaries were also changed without independent version gates:

- `imageagent.execute_slot.v2` was reused after its external-effect semantics changed.
- approval publication changed from a plan-derived idempotency key to the command `ActionID` without a workflow version marker.

Separately, the final ListingKit transaction accepts more than one main-image candidate and applies the last one, while the public run DTO omits the persisted `max_concurrent_slots` value.

These findings share one root cause: the system has no single, immutable protocol describing wire version, artifact durability, effect ownership, reconciliation, and final-result invariants. Fixing each call site independently would leave the same class of error available at the next retry or deployment boundary.

## 3. Goals

- Preserve deterministic replay for workflow histories created before this design.
- Never mark generated output recoverable while it depends on a worker-local file.
- Recover S3-compatible writes whose success response was lost.
- Serialize publication with a database claim, lease, and fencing token.
- Produce one durable business result under retries, crashes, and lease handoff.
- Block ambiguous non-reconcilable outcomes instead of repeating a potentially successful side effect.
- Persist only an explicit metadata allowlist.
- Require exactly one approved main-image candidate.
- Expose effective concurrency and actionable blocking state to the workbench.
- Reuse the repository's Temporal, AWS SDK v2, S3 uploader configuration, projection store, and ListingKit transaction boundaries.

## 4. Non-goals

- Do not add Agent planning, autonomous repair, or non-image workflow nodes.
- Do not store image bytes in the application database.
- Do not use a PVC or assume that a retry runs on the same Pod.
- Do not promise physical exactly-once invocation for providers that offer neither idempotency nor result lookup.
- Do not automatically regenerate after an ambiguous provider outcome.
- Do not migrate an in-flight v2 workflow history to v3 in place.
- Do not perform synchronous staging-object deletion in a user request.
- Do not redesign ProductImage providers or create a second object-storage client.

## 5. Ownership and dependency direction

The recovery protocol belongs to `internal/imageagent` and is expressed through domain ports. It must not import the AWS SDK, GORM, Temporal, HTTP, or ListingKit implementation packages.

- Temporal owns durable orchestration, wire-version choice, and activity scheduling.
- `imageagent` owns slot-effect identity, phases, immutable manifests, claims, fencing, and transition validation.
- `imageagent/store` owns transactional compare-and-swap transitions for memory and GORM adapters.
- ProductImage adapters own provider generation and conversion between generated assets and image-agent artifacts.
- The object-storage adapter owns staging, HEAD reconciliation, and deterministic finalization using the existing AWS SDK v2 client and configured S3/COS endpoint.
- ListingKit owns the atomic application of the fully approved candidate set to the standard product.
- HTTP projections expose state but do not infer or repair it.

The dependency direction is:

```text
Temporal / HTTP / ProductImage / S3 adapters
                    ↓
            imageagent ports and model
                    ↓
        repository transition contracts
```

## 6. Temporal compatibility protocol

### 6.1 Immutable activity names

Existing activity names remain registered with handlers compatible with their historical payload and result shape. They are never rebound to the new v3 receipt semantics.

New executions use distinct names:

- `imageagent.execute_slot.v3`
- `imageagent.publish_approved.v3`

Other activity names stay at their current version unless their payload or effect semantics change during implementation. A version suffix is a wire contract, not a release label.

### 6.2 Independent workflow version gates

The workflow adds separate `workflow.GetVersion` patches for independent decisions:

1. Slot execution wire selection:
   - histories without the marker select the exact v2 activity route;
   - new histories select `execute_slot.v3`.
2. Approval idempotency-key selection:
   - histories without the marker retain the historical plan-derived key;
   - new histories use the command `ActionID`.
3. Approval publication wire selection:
   - histories without the marker retain `publish_approved.v2`;
   - new histories select `publish_approved.v3`.
4. Result-digest algorithm selection:
   - histories without the marker retain the historical candidate-ID-only digest;
   - new histories use the v3 digest covering plan revision, role, durable object identity, and content hash.

These decisions must not be collapsed into the existing v2 patch. Each marker has one meaning so future maintenance cannot accidentally change an unrelated replay branch.

Every new command `ActionID` is also a resume-route path segment. Command ingress, workflow validation, and the ListingKit BFF therefore share one canonical ASCII contract: 1-128 bytes, beginning with an alphanumeric character, followed only by alphanumerics or `._:+-`. The client percent-encodes that value for transport. Identifiers that require path separators, whitespace, query delimiters, or non-ASCII normalization are rejected before they can become durable pending commands.

### 6.3 Real-history replay

Tests commit serialized workflow histories captured from the pre-change code paths. Replay must cover:

- an in-flight v2 slot activity;
- a completed slot followed by approval;
- an approval command whose old key is plan-derived;
- an awaiting-approval history whose stored result digest uses the historical algorithm;
- workflow histories both with and without the existing atomic-command v2 marker.

Constructing a new history using only the new workflow implementation is not sufficient evidence of compatibility.

## 7. Durable artifact model

### 7.1 Staged asset reference

The database stores an immutable `StagedAssetRef` for each generated asset:

```text
object_key
sha256
size_bytes
content_type
width
height
source_asset_id
operations
provider_receipt_id
```

Only these fields are eligible for persistence or API projection. `local_path`, credentials, arbitrary provider metadata, internal headers, and transient URLs are rejected rather than silently copied.

### 7.2 Deterministic object identity

Each staging object uses an identity derived from the owner scope and content:

```text
image-agent/staging/{tenant}/{run}/{planRevision}/{slot}/{attempt}/{assetIndex}-{sha256}.{ext}
```

All path segments are validated canonical identifiers. The content extension comes from an allowlisted content-type mapping, not from an untrusted filename. The PutObject request records SHA-256 and size metadata. When the endpoint supports server-side checksum fields, the adapter requests and verifies SHA-256 directly. Otherwise reconciliation requires the deterministic content-addressed key, matching application metadata, exact content length, and an immutable-key write policy. A plain ETag is never treated as SHA-256 because multipart and S3-compatible implementations do not guarantee that meaning.

The corresponding final object also has a deterministic key. Finalization may copy a staging object or recognize that the staged object already satisfies the durable public-asset contract. It must not create a random destination name during a retry.

Generated bytes may initially be represented by a provider-local file or by an explicitly allowlisted provider result reference. The ProductImage adapter resolves that transient representation while the owning activity is alive and hands bytes or a bounded reader to the staging port. The transient representation is never part of `StagedAssetRef`.

Before persisting `staging_prepared`, the storage adapter writes the complete generated byte set as one immutable, identity-bound recovery bundle under the same `image-agent/staging/` lifecycle namespace. The bundle contains only the normalized manifest and its verified image bytes; it never contains local paths, credentials, arbitrary provider metadata, or transient provider URLs. Its deterministic attempt key, content checksum, bounded read, exact manifest check, and staging lifecycle policy make it a recovery boundary rather than a second publication model.

### 7.3 Storage port

The domain-facing storage port supports:

- prepare and validate deterministic object identities;
- preserve and rehydrate an immutable attempt-scoped recovery bundle;
- put an immutable object;
- inspect an object including content metadata;
- finalize a staged object to its deterministic public identity;
- reconcile an uncertain put or finalize operation.

The adapter extends the existing S3 infrastructure around AWS SDK v2 `PutObject`, `HeadObject`, bounded `GetObject`, and, when needed, `CopyObject`. It does not implement an S3 protocol or a separate credential/configuration path.

## 8. External-effect state machine

### 8.1 Phases

The v3 state machine is:

```text
provider_claimed
  → staging_prepared
  → artifact_staged
  → publication_claimed
  → publication_complete

provider_not_dispatched
  → provider_claimed
```

Explicit blocked outcomes are:

- `provider_outcome_unknown`
- `staging_outcome_unknown`
- `publication_outcome_unknown`

A blocked outcome is durable and actionable. It cannot be converted back to an executable phase by an automatic activity retry. A user command creates a new slot attempt when retry is safe.

### 8.2 Provider claim

The attempt identity is `(tenant, owner user, run, plan revision, slot, attempt)`. A compare-and-swap reservation creates one provider owner for that identity and binds it to the input fingerprint and idempotency key.

When budget authorization is active, only an attempt with no persisted effect asks the selected provider route for a quote. The reservation persists that quote and policy as part of the effect contract. Every later activity attempt loads the effect before consulting current provider configuration and reuses the persisted quote; a rollout, routing change, expired provider deadline, or missing quote capability must not prevent staging or publication recovery for work that has already crossed the provider boundary.

If an activity retry observes `provider_claimed` but did not create the claim, it must not invoke generation again. It may continue only by loading the verified recovery bundle for that exact attempt; an absent or conflicting bundle becomes the typed `provider_outcome_unknown` block.

`provider_not_dispatched` is the only provider-side state that permits the same attempt to claim generation again. It is written only after the provider adapter explicitly proves that it did not dispatch a request (including a local rejection before effect). If a budget quote was reserved, that transition releases it atomically; the subsequent claim reauthorizes the same persisted quote. All timeout, transport, and unclassified provider errors remain `provider_outcome_unknown`.

### 8.3 Staging preparation and upload

After provider generation returns in the owning activity:

1. Read each generated file while it still exists.
2. Validate type and dimensions, compute SHA-256 and size, and sanitize metadata.
3. Build the complete immutable staging manifest.
4. Preserve the complete byte set in one immutable attempt-scoped recovery bundle.
5. Persist `staging_prepared` with a compare-and-swap from `provider_claimed`.
6. Upload each object under its deterministic key.
7. Confirm every object with HEAD metadata.
8. Commit `artifact_staged` with a compare-and-swap from `staging_prepared`.

If the process fails before the recovery bundle is durable, the provider outcome is unknown and the owning attempt retries bundle preservation with bounded backoff while its worker-local files are still available. Once that bounded retry is exhausted, the activity cleans those local files and durably blocks as `provider_outcome_unknown`; no later Temporal retry may assume those bytes still exist. Once preservation succeeds, those local files are deleted before any further retryable boundary. A retry can then rehydrate the exact manifest and bytes even when `staging_prepared` was not committed or individual uploads are incomplete. Matching objects are reconciled without rewriting; missing objects are uploaded from the bundle; conflicting objects or a missing/conflicting bundle fail closed as an unknown outcome.

### 8.4 Publication claim and fencing

Publication is a separately claimed effect. A successful claim records:

- owner identifier;
- lease expiry;
- monotonically increasing fencing token;
- publication fingerprint;
- deterministic final-object manifest.

Lease comparison and token allocation use database time inside the claim transaction, not a worker's local clock. The owner may renew an unexpired lease with the same token while it is active; takeover after expiry always allocates a higher token.

Only the current fencing token can commit `publication_complete`. A lease successor receives a higher token. The old owner may finish a delayed network call, but cannot change database truth. Deterministic final keys and content checks ensure both owners converge on the same observable object rather than creating duplicate business assets.

The claim prevents concurrent normal execution; deterministic destination identity and reconciliation handle the unavoidable network window beyond the database transaction. For a destination with neither deterministic identity nor provider idempotency, lease expiry does not authorize an automatic takeover after a request was sent. The attempt becomes `publication_outcome_unknown` and requires reconciliation or operator action.

### 8.5 Publication reconciliation

After an uncertain finalization response, the owner inspects the deterministic final object and verifies SHA-256, size, and content type. A matching object is success. A missing object can be retried while the claim is current and the staging object is verified. A conflicting object or an external destination that cannot be queried becomes `publication_outcome_unknown`.

The system does not use a timeout alone as proof that publication failed.

## 9. Approval and standard-product invariants

Approval is accepted only when all of the following are true:

- the run and plan revision still match the command;
- every declared plan slot is accepted;
- the ordered requested candidate IDs exactly match the complete projection;
- candidate IDs are unique and every candidate references a durable published object;
- there is exactly one main-image candidate across the complete result;
- every other approved candidate has a non-main image role;
- the result digest matches the projection.

A main-role slot that returns zero or multiple candidates is rejected when its slot result is persisted. Final approval validates the same invariant again at the ListingKit transaction boundary. It never selects the last main candidate implicitly.

The v3 digest covers plan revision and, in declared slot order, slot ID, role, candidate ID, durable object key, and content hash. Changing any approved content or ordering invalidates approval. The workflow records an independent digest-algorithm version marker before computing this value; old histories and the frozen v2 approval handler continue to use the historical candidate-ID-only digest.

Applying asset records, selecting the main asset, updating gallery selection, and writing the publication acknowledgement remain one ListingKit database transaction. Idempotent replay returns the stored acknowledgement only when its fingerprint is identical.

## 10. Public projection and workbench behavior

The run DTO and frontend `ImageAgentRun` include `max_concurrent_slots`, using the normalized persisted run value as the source of truth.

Blocked v3 effects project stable reason codes and permitted actions:

- `provider_outcome_unknown`: explain that generation may have happened and permit cancellation only. Automatic retries, plan edits, and replacement attempts are forbidden because they can duplicate a paid provider effect. A future reconciliation action needs a provider-neutral result lookup/idempotency contract and explicit UI/authorization; it is intentionally not simulated by a retry action.
- `staging_outcome_unknown`: explain that durable bytes are incomplete and offer a new manual attempt.
- `publication_outcome_unknown`: require an operator or user to verify the destination before another publication command.

The projection persists these states, so page refresh and event-stream reconnect do not lose the explanation. The frontend must not infer a generic spinner from a non-terminal run when the current effect is blocked.

## 11. Deployment and migration

Migration is additive and follows this order:

1. Add v3 effect columns or tables, indexes, and constraints without rewriting v2 rows.
2. Deploy a worker build that registers frozen v2 handlers and the new v3 handlers.
3. Start the worker on a dedicated v3 task queue. Existing v2 workers do not poll it.
4. Verify worker readiness and execute a v3 canary workflow.
5. Enable API routing of newly created runs to the v3 queue.
6. Deploy the API and frontend projection changes.
7. Keep the v2 queue and compatible worker available until all existing histories drain.

An in-flight v2 run is never rewritten to v3. Before replacing any existing v2 handler deployment, operations must query Temporal for open v2 workflows and activities. Ambiguous historical activities are surfaced for manual handling rather than silently replayed with new semantics.

Rollback stops routing new runs to v3 but keeps the v3 worker available for already-started histories. A v2-only worker must never poll the v3 queue.

Staging cleanup is configured as an object-store lifecycle policy on the staging prefix with a retention period longer than the maximum workflow recovery window. Application request paths do not synchronously delete staging objects in phase one.

## 12. Test strategy

### 12.1 Repository contracts

Memory and GORM repositories run the same state-transition suite:

- one provider owner per attempt;
- immutable input fingerprint and staging manifest;
- legal phase order only;
- exact replay accepted and conflicting replay rejected;
- one current publication owner;
- monotonic fencing tokens;
- stale-token completion rejected;
- tenant and owner isolation on every operation.

### 12.2 Storage fault injection

Tests cover successful upload, lost PutObject response, partial upload, missing object, metadata mismatch, matching pre-existing object, conflicting final object, and restart after `artifact_staged`. Persisted JSON and HTTP output are scanned to ensure forbidden metadata and local paths are absent.

The S3 adapter is tested through the AWS SDK client boundary. An optional MinIO integration test may validate real S3-compatible behavior, but correctness cannot depend only on a mocked happy path.

### 12.3 Concurrency and crash boundaries

Tests pause two activities at each compare-and-swap boundary. They verify that only the current owner can commit, a stale owner is fenced after lease handoff, and deterministic keys converge on one observable final asset even if a delayed call returns.

Crash tests cover every boundary between provider return, staging preparation, object upload, staged commit, publication claim, finalization, and publication completion.

### 12.4 Approval and API

Approval tests cover zero, one, and multiple main candidates; missing, duplicate, reordered, or changed candidates; changed hashes; and stale plan revisions. API and frontend tests verify effective concurrency and all blocked-state actions after refresh or event reconnect.

### 12.5 Verification commands

Implementation verification includes focused domain, repository, Temporal replay, activity, ListingKit transaction, HTTP, and frontend tests, followed by:

```text
go test ./...
frontend typecheck and test commands defined by the workspace
deployment manifest and configuration tests
```

Passing tests are necessary but do not replace a v3 canary, old-history replay, or runtime drain check.

## 13. Acceptance criteria

- A run at `artifact_staged` completes after its original Pod and temporary directory are removed.
- No durable record or API response contains a Pod-local path or unapproved provider metadata.
- Committed old histories replay without nondeterminism.
- Newly created runs schedule only v3 slot and approval activity wires.
- Concurrent retries and lease handoff produce one durable observable asset result.
- An ambiguous external result blocks with an actionable reason and is not automatically repeated.
- Approval cannot create zero or multiple selected main assets.
- `max_concurrent_slots` is visible and equals the normalized persisted value.
- The workbench explains why a task stopped and which action is permitted.
- Focused tests, full Go tests, frontend checks, deployment checks, and a v3 canary pass before rollout is considered accepted.

## 14. Phase-one boundary

Phase one includes manual image workflow v3 recovery, S3/COS durable staging, publication claim and fencing, Temporal compatibility migration, final approval invariants, effective-concurrency projection, and actionable blocked states.

Agent planning, automatic repair, database blobs, PVC storage, synchronous object cleanup, and exactly-once claims for non-reconcilable external platforms remain outside this slice.

## 15. Release-authority and drain addendum (round-4 design correction)

Repository text and shell regexes are not a production authorization boundary.
The production release path therefore uses two distinct GitHub environments,
`listingkit-api-production` and `listingkit-ui-production`. Each job requests a
short-lived GitHub OIDC token with a distinct audience and never consumes a
long-lived kubeconfig credential. The Kubernetes API server is configured once
to trust `https://token.actions.githubusercontent.com`; RoleBindings bind only
the exact repository/environment subjects, and separate least-privilege Roles
limit the API and UI owners to their named resources and required verbs.
The API server and local fail-fast check bind ordinary workflow tokens through
GitHub's standard `workflow_ref` claim to the exact repository, workflow path,
and `refs/heads/main` ref for the API or UI identity; path-prefix, feature,
tag, and missing-ref claims fail closed. They do not require
`job_workflow_ref`, which GitHub documents only for called reusable workflows.
Production release fails closed until this external trust and RBAC prerequisite
is installed. Repository deployment workflows never bootstrap or widen their
own authority.

The API release previously created schema-migration, identity-preflight, and
canary Jobs dynamically. Kubernetes RBAC cannot constrain a top-level `create`
request with `resourceNames`, so granting `create jobs` would reopen the same
authority gap. Those gates therefore run as init containers in four
administrator-installed, zero-replica Deployments. The release identity may
patch only those named Deployments. Each invocation client-renders the reviewed
aggregate manifest, selects one exact zero-replica Deployment, verifies that a
live object already exists, reapplies only that selected object, patches only
the `release-gate` init-container image and a fixed Pod-template invocation
annotation derived from the trusted GitHub run ID, run attempt, and runner
name, and scales it to one. Before success,
the helper compares a deterministic live projection of command, arguments,
credential/config scope, service-account settings, volumes, security contexts,
resources, images, selectors, and labels against the reviewed object after the
single image, invocation annotation, and replica changes. It then requires the
current Deployment generation, one updated replica, one available replica, and
zero unavailable replicas. The changed template makes this a proof for the
current invocation's init completion without listing Pods. A digest-pinned
`pause` container holds the successful Pod, and the driver always scales the
runner back to zero. Production release
Roles contain no top-level `create`, wildcard, or RBAC-management verb. Initial
workload/RBAC installation remains an explicit administrator prerequisite.

One machine-readable release-policy document is the source of truth for OIDC
issuer/audiences/subjects, protected Kubernetes resource identities, verbs,
GitHub workflow/job/step owners, and RBAC manifest locations. Pinned
OPA/Conftest evaluates structured workflow, policy, and RBAC YAML. Actionlint
continues to validate workflow syntax. Supported Markdown is parsed with a
Markdown AST across every fenced block to reject executable direct-production
examples and link the canonical workflow invocation. CI path filters include
the supported README; prose lint is defense in depth and is never reported as
authorization. The handwritten workflow/helper/prose mutation grammar is
removed.

Every API image eligible for this release path carries the immutable OCI label
`org.opencontainers.image.listingkit.image-agent-routing=image-agent-v3-new-starts-v1`.
The API workflow inspects that exact label for built and supplied digests before
any production mutation, exposes no caller override, and stamps the same fixed
contract beside run ID, run attempt, and image on the Deployment and Pod
template. A prior v2-producing digest is ineligible for this release path.

Retiring the v2 worker requires a stable three-sample quorum, not one Visibility
query. Every sample must re-prove all of the following:

1. the serving API Deployment and every API Pod carry the exact attested
   `run_id`, `run_attempt`, digest, and v3-new-starts routing annotations and no
   old/mixed or v2-producing image exists;
2. a parameterized authoritative database query returns zero non-terminal
   `image_agent_v2_runs`, preserving full `(tenant_id, owner_user_id, id)`
   identities for any nonzero/manual reconciliation result;
3. the paired Temporal CLI 1.8.1 count/list evidence for parent and child types
   agrees, and exact describes contain no v2 queue child/activity work; and
4. the fixed 300-second convergence interval has elapsed before the next
   sample.

Any producer, database, Temporal, parsing, command, or cross-source disagreement
is nonzero evidence. The final sample repeats producer quiescence, so Visibility
zero is only a convergence signal corroborated by serving-image and database
authority. The exactly three complete samples are separated by two 300-second
waits, yielding a 10-minute first-to-final proof window. This duration is a
ListingKit operational policy, not a Temporal SLA. Tests replace `sleep`
through `PATH` while retaining the production interval value; production has no
sample-count or zero-interval override.

## 16. Final round-5 closure

The trusted workflow ref, immutable v3 producer label/stamp, reviewed live
runner projection, and all-fence README lint close the four final review gaps
as one release contract. None is a live rollout claim: production drain,
canary, deployment, Temporal state, and business acceptance remain separate
operator evidence and require explicit authorization outside this task.
