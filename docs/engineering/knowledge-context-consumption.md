# Knowledge execution context local ports (#558)

Design Basis: Reuse Existing Architecture, [approved contract](../architecture/agent-knowledge-context-v1.md) / [#558](https://github.com/qq550723504/task-processor/issues/558).

This slice supplies the current Knowledge owner with immutable execution context,
exact revision citations and a local dispatch fence. Product Agent prompt/Request,
Human Review and browser integration belong to #559. Uploading a document still
does not send it to a model. There is no public retrieval endpoint or Commerce Tool.

## Assembly and calling sequence

Borrow the existing `knowledge.Service` prepared from the dedicated Knowledge
runtime pool. Create `knowledgeauth.NewAuthorizer(organizationResolver,
listingKitAuthorizer)`, then `knowledgeService.Context(authorizer)`. The adapter
reuses Workbench `Resolve(LiveWrite)`, the fresh principal resolver and the existing
`workbench.knowledge.read` Casbin policy. It owns no IAM policy or cached authority.

Install the verified request-local `commercetoolauth.OrganizationRequest` on each
operation's context, using the current actor/token/selected Organization. Do not
persist its bearer token. A `Scope` or checkpoint identity alone is insufficient:
missing/expired credentials, revoked grants, changed actor/Organization and a
viewer role fail closed. Read permission does not authorize Product Agent execution.

1. Construct `knowledge.ContextRequest` server-side from verified scope, the exact
   existing `agent.Binding`, stable Agent request key, one normalized
   `knowledge-base:<canonical-lowercase-uuid>` selection and `ContextPolicyVersion`.
2. Call `Materialize` before Agent Start. The first commit resolves every ACTIVE
   source's current readable revision. It returns only `ContextSnapshotRef{kind,id,digest}`.
3. The unique identity is Organization + actor + Binding context kind/ID + request
   key. The fingerprint includes the full Binding, selection and policy, excluding
   server-resolved source/revision/digest/count. Retry after a lost response or
   process restart adopts the original ref, including after newer revisions or
   sources are added. Changed Binding/selection/policy conflicts.
4. `ReadContext(scope,ref)` obtains the exact frozen payload only after live
   permission and current Base/Source/fence/readable-revision checks. A newer
   current-readable pointer does not invalidate an older exact readable revision.
   Store only the opaque ref in Agent/checkpoint/browser state, never this payload.
5. `ReadCitation(scope,ref,citationID)` performs the same admission and resolves only
   inside that exact bundle. Unknown IDs, another bundle's citation or wrong digest
   fail. The returned name/text/warning are protected content, not Product fact,
   enrichment Evidence, authorization or approval. Each V1 source has one citation
   at safe location `text`; no parser-invented section is trusted.

No selection continues the consumer's existing no-Knowledge path; do not call this
materializer with `none` and do not create an empty bundle. At least one and at most
four ACTIVE sources must all have an AVAILABLE/PARTIAL revision. PARTIAL carries its
original warning. Pending/FAILED-only sources return `KNOWLEDGE_NOT_READY`.

Limits are hard: 16 KiB normalized text per revision, 48 KiB total text, four sources,
64 citations and 96 KiB serialized payload. V1 has one citation per source, hence
the four-source cap is stricter than the citation cap. Oversize fails with
`KNOWLEDGE_CONTEXT_TOO_LARGE`; no truncation, first-N selection or fallback occurs.

## Dispatch permit and disable

The future #559 consumer must acquire a `DispatchPermit` in the existing
`BeforeDispatch` seam, after queueing and immediately before transport. Acquisition
requires fresh admission, exact ref/digest/revisions/fence versions and ACTIVE
lifecycle rows. It commits before network I/O and holds no database transaction
over a provider call. Each Organization + InvocationID can be acquired once;
duplicate/terminal identities conflict and never renew or authorize redispatch.

The fixed database-clock lease is 5m30s. The existing text provider rejects timeout
configurations above 5m; the admission regression test exercises that boundary
without calling a provider. Release after CompleteText returns for every outcome,
using a bounded cleanup context even after cancellation/revocation. Release is
bound to the returned permit identity and does not reveal content or write AI outcome.
AI Capability remains the provider/usage/UNKNOWN owner.

The existing disable endpoints return 202 + DISABLING while an earlier live permit
drains, or 200 + DISABLED when none remains. Both states reject subsequent reads,
materialization, citation content and new permits immediately. Release and the
existing five-second Knowledge processor sweep finalize drained/expired disables.
Recovery only expires permits and advances Knowledge lifecycle; it never calls a
model or changes AI success/failure/UNKNOWN. Historical opaque IDs remain intact.

## Storage, fresh installation and verification

All seven tables live in the dedicated `knowledge` database. Bundle payload bytes,
digest, command identity/fingerprint and entries are immutable to `knowledge_runtime`;
permit UPDATE is limited to its state, excluding invocation binding and lease.
Serving construction verifies all tables/grants and performs no DDL.

Use the existing [Knowledge startup instructions](../../deployments/docker/account-compose/KNOWLEDGE.md)
with a new empty Compose project. Schema-init installs the complete current schema.
An older initialized Slice A project is not upgraded in place; its serving
constructor will reject missing Slice B tables. Keep retained trial volumes and
their matching image/checkout together. This change authorizes no migration,
redeployment, retained-instance rebuild or data deletion.

```powershell
go test ./internal/knowledge/... ./internal/integration/knowledgeauth -count=1
go test -tags integration ./internal/integration/persistence/knowledge -count=1
go test -tags integration ./internal/app/httpapi -run '^TestCurrentKnowledgePostgresLiveAuthorizationAndRestart$' -count=1
go test ./internal/integration/openai -run '^TestTextRouteTimeoutFitsKnowledgeDispatchPermitLease$' -count=1
```

PostgreSQL tests use isolated containers and synthetic documents. Reconstructing a
repository tests durable adoption; controlled expiry tests recovery. These do not
claim an actual OS process crash, real enterprise document acceptance, paid model
calls, full browser use, or production deployment. Those remain NOT_RUN here.
