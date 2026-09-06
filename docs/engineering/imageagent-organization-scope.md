# ImageAgent Organization scope admission

Refs #339; consumer #334. Production assembly stays disabled. This scope does not enable invocation recording, change model retries, migrate historical data, or replace Temporal recovery.

## Frozen narrow contract

- Explicit application constructor only, patterned after the current Product Review application. Direct run routes only: create, get, plan update, restart, retry, cancel, resume, events and existing approval/recovery commands. No task-runs routes or ListingKit input owner. GET uses CachedRead; every write uses LiveWrite. Existing verified identity, workbench resolver and ImageAgent read/write permissions execute before the service.
- New organization-mode service requires a verified EffectiveOrganizationID equal to transport TenantID and a verified actor. No body identity fields are accepted. Existing owner-only service rules remain: another actor, including an administrator, cannot rewrite or take over a run.
- Scope protocol `image-agent.organization.v1` is persisted with the run and execution identity. Existing TenantID/UserID are the only organization/owner values, not parallel independently mutable fields. Identity also binds RunID. The run is the durable authority; workflow/activity identity must match it. Home is not persisted as an execution owner.
- Explicit organization repository mode validates protocol on create/read/commit. Historical repository mode refuses organization protocol runs; organization mode refuses missing/unknown protocol. Initialization remains one existing PostgreSQL transaction. Same org/owner/key changed payload conflicts; cross-org reuse never retrieves another org's run. No migration/backfill or default schema switch.
- New Temporal wire selection uses its own task queue and workflow registration name while executing the existing workflow and activity bodies. Default v2/v3 paths reject organization identity; organization activity admission rejects legacy identity. Parent input and children carry the same identity. No extra state machine, reconciler or retry scheduler. Existing initialization/start recovery and USE_EXISTING semantics remain in their owner.
- Before an organization Activity proceeds, load the matching persisted run, verify protocol, RunID/org/original actor and business context, query fresh actor grants for that org/project, and apply existing ImageAgentWrite authorization. Missing grant, revoked grant or failed lookup rejects before assets, credentials or provider calls. Do not persist JWT/cookies/service tokens or roles.
- User authorized the minimal read-only service authorization adapter in this task: reuse the existing ZITADEL ListAuthorizations pagination/validation in authruntime, expose a separately named service-authorized subject query, preserve ListOwnProjectAuthorizations's user-token-only contract. Service credentials come only from explicit assembly. No role grant/mutation or provisioning runtime dependency.
- On restoration, authenticated/AI execution context uses the persisted org and original actor after the live authorization check. Organization credential selection must not fall back to another org or an unapproved static route. Asset catalog admission uses org/owner and a controlled owned current catalog source, not an arbitrary TaskID-as-business-system claim.
- Existing resource limits, deadlines, quotes and cost-unknown rejection are unchanged. New HTTP assembly uses existing strict JSON bounds and a bounded request timeout. Authorization HTTP uses the current bounded response/pagination/client timeout behavior. A failed authorization attempt is not permission to continue under a cached role snapshot.

## Persistence and failure boundaries

The current InitializeRun transaction owns run/plan/catalog/projection/event creation. Failed transaction leaves no admitted run. Response loss retries the same idempotent initialization/start identity; changed payload is rejected. Service reconstruction re-reads the persisted run. Temporal start failure leaves the existing initialized run recoverable through the current restart path, not another run or ledger.

Workflow serialization uses Temporal's actual converter. Activity authorization is a separate current permission check; neither a token expiry fabricated from a persisted identity nor a role snapshot can satisfy it. Authorization failure does not rewrite durable owner. Existing cancelled/expired activity context continues to fail closed.

Legacy run/history evidence remains untouched. A new-mode reader does not reinterpret an old row as an organization row; a new-mode worker refuses an old/mixed envelope. Already deployed old binaries cannot be retroactively guarded by code: stopping old writers and enabling new runtime credentials/queues are explicit production gates, never assumed from isolated tests.

## Verification and scope

TDD must cover actual HTTP middleware/resolver/authorizer -> run service -> isolated PostgreSQL -> reconstruction -> Temporal converter/test environment -> actual Activity admission -> real Manager/credential resolver/Review adapter -> loopback HTTP. Identity issuance, external grants and external provider responses may be controlled substitutes; the final organization cannot be injected into the Review test context.

Positive Home A/selected B and negative missing org, grants revoked, failed dependency, actor switch/admin, scope tamper, wrong assets/credentials, old protocols and cross-mode access are required. Count workflows/provider calls on denied paths; verify stored owner and serialization after reconstruction. Temporal SDK test environment is not a real server/restart acceptance claim. Paid models, real IAM, production and migrations are NOT_RUN.

Expected implementation owner paths: app HTTP explicit assembly/tests; ImageAgent scope/service/model, HTTP descriptors, GORM run codec/projection checks; Temporal wire/Activity identity admission and tests; authruntime service-read adapter/tests; narrowly scoped OpenAI credential selection and worker scope-consumption tests. Limit to at most 30 related files and the existing production-code limits; if exact implementation cannot fit, split before exceeding them.

Review state: IMPLEMENTATION_READY after independent narrow review. No BLOCKER found. Required implementation tests cover parent/child/recovery isolation, repository sibling paths, cross-organization idempotency conflicts, current grant revocation, and persisted/converter scope agreement. The task authorizes implementation against this frozen contract; production remains disabled.

## Assembly and consumer handoff

`app/httpapi.NewImageAgentOrganizationApplication` accepts the existing verifier,
organization resolver, authorizer, an organization Temporal client, a mandatory
tenant start gate, and immutable `OrganizationImageBinding` values. Bindings
qualify a current Catalog publication/version by organization, original actor,
and ContextID. ContextID is carried by the existing BusinessTaskID field; it is
not a new Task system. The application neither publishes Catalog nor migrates
the database. Tests explicitly initialize a fresh database with
`store.AutoMigrateOrganizationScope`.

The prefix is `/api/organization/image-agent`. Routes are POST `/runs`; GET
`/runs/:run_id`; POST `/runs/:run_id/restart`; PUT `/runs/:run_id/plan`; POST
`/runs/:run_id/slots/:slot_id/attempts/:attempt/recover`; POST
`/runs/:run_id/slots/:slot_id/retry`; POST `/runs/:run_id/results/approve`; POST
`/runs/:run_id/cancel`; GET `/runs/:run_id/events`; POST
`/runs/:run_id/commands/:action_id/resume`. The assembly excludes task-runs.

Use `temporal.NewOrganizationClient`, `WorkerWireModeOrganization`, and
`OrganizationTaskQueue` (`image-agent-organization-v1`) together. Parent workflow
registration is `ImageAgentOrganizationWorkflowV1`; parent/child/recovery IDs
have an organization protocol namespace. `ActivityDependencies.ExecutionAuthorizer`
must be present on that worker and absent on historical workers. Restore verifies
`ExecutionIdentity{ScopeProtocol, RunID, TenantID, UserID, BusinessTaskID}` against
the persisted run, then establishes both authenticated and AI contexts. It does
not reinterpret or persist Home, roles, or tokens. Catalog content is bound by
the existing canonical hash/version; storage timestamp precision is not an
authorization attribute.

`app/worker/imageagent.OrganizationExecutionAuthorizer` reuses ZITADEL's bounded
authorization reader with explicitly supplied service credentials and the
existing ImageAgentWrite policy. Configure the same optional deny-only
organization status checker used by HTTP, if present. The adapter performs no
grant or identity mutation. `BuildOrganizationImageCapabilities` takes a dedicated
Manager and installs `NewOrganizationCredentialResolver`: only same-org user or
organization credentials, no static fallback. The existing routed Review,
quoting and ProductImage adapter remain the consumer owned by #334.

The integration test is `TestOrganizationScopeHTTPPersistenceAndActivity` in
`internal/app/httpapi/imageagent_organization_integration_test.go`, enabled only
with `ISSUE339_TEST_DSN` for a fresh task database. Its existing staged artifact
and generation receipt are synthetic fixtures. Review itself uses actual
budget authorization, quote, credential lookup and adapter HTTP; the provider
returns needs-human-review so no image is generated, approved or published.
Temporal server acceptance/USE_EXISTING is a controlled server boundary;
converter and SDK Activity execution are real. The mixed-wire tests execute
the actual parent, child and recovery workflows in the SDK test environment.
This is not evidence of a real Temporal server restart.

Before production: separately authorize inventory/cutover, stop historical
writers, provision and verify the least-privilege service credential, configure
current bindings/gates/worker queue and any organization suspension policy, and
validate the real environment. Before #334 resumes against main, coordination
must approve and merge this prerequisite. #334 still owns InvocationRecorder,
record-write failures, unknown-cost handling and actual retry ownership/HTTP
counts; none are declared complete by this scope handoff.
