# OpenMeter Shadow Metering PoC Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development for every behavior change, superpowers:systematic-debugging for any unexpected failure, and superpowers:verification-before-completion before claiming the PoC complete.

**Goal:** Build an isolated, reproducible OpenMeter contract PoC that decides whether ListingKit should adopt OpenMeter for metering and entitlement without changing any production runtime, billing, payment, deployment, or existing quota behavior.

**Architecture:** Add a non-bootstrapped adapter under `internal/integration/openmeter` around the official v3 Go SDK, drive a pinned upstream OpenMeter quickstart from PowerShell, and express the three catalog metrics as real contract tests. The repository remains the owner of business-success facts and stable event identity; OpenMeter is evaluated only as the generic event aggregation and access engine. All real-environment tests are opt-in and fail closed once opted in.

**Tech Stack:** Go 1.26, `github.com/openmeterio/openmeter/api/v3/client` `v1.0.0-beta.231`, OpenMeter OSS `v1.0.0-beta.232`, CloudEvents 1.0, Docker Compose, PowerShell/Pester, existing Go test infrastructure.

## Global Constraints

- Treat [`docs/superpowers/specs/2026-08-13-openmeter-shadow-metering-poc-design.md`](../specs/2026-08-13-openmeter-shadow-metering-poc-design.md) as the behavioral authority.
- Do not modify `cmd/`, application bootstrap, HTTP handlers, workflows, consumers, database schema, Kubernetes manifests, existing `CheckUsage`/`AuthorizeUsage`/`RecordUsage`, or payment/billing code.
- Use only the official v3 SDK module for OpenMeter API calls. Do not import the legacy root `github.com/openmeterio/openmeter` module, write OpenMeter HTTP DTOs, or add a parallel REST client.
- Pin service and SDK independently. The service is `v1.0.0-beta.232`; the latest published v3 SDK at design time is `v1.0.0-beta.231`. Compatibility must be proven by contract tests, not inferred from version numbers.
- The upstream quickstart contains `latest` image references. The runner must override every OpenMeter-owned service to `ghcr.io/openmeterio/openmeter:v1.0.0-beta.232` and record the resolved digest.
- Keep the quickstart checkout and raw evidence under ignored `.local/openmeter-poc/`; never copy its Kafka, ClickHouse, PostgreSQL, Redis, or Svix Compose definitions into tracked project files.
- Default `go test ./...` must not contact OpenMeter. Contract tests skip only when `OPENMETER_POC` is absent; after `OPENMETER_POC=1`, missing URL, unhealthy dependencies, or setup failure is a test failure.
- Use deterministic fixture keys derived from `OPENMETER_POC_RUN_ID`; do not delete shared Docker volumes or resources outside that namespace.
- Never log API keys, connection strings, customer data, ZITADEL tokens, marketplace credentials, or product content.
- Commit after each task only when its focused tests pass. Stage only paths named by that task.

## File Map

| Path | Responsibility |
| --- | --- |
| `go.mod`, `go.sum` | Pin only the official OpenMeter v3 client module. |
| `tests/openmeter_poc_boundary_test.go` | Enforce exact dependency and keep OpenMeter out of production bootstrap/config/deployments. |
| `internal/integration/openmeter/doc.go` | Document the isolated PoC boundary. |
| `internal/integration/openmeter/usage_event.go` | Validate product metrics and build stable official-SDK `EventInput` values. |
| `internal/integration/openmeter/usage_event_test.go` | Pure event-contract tests; always part of the default suite. |
| `internal/integration/openmeter/client.go` | Thin official-SDK wrapper for ingest, meter query, and access query. |
| `internal/integration/openmeter/errors.go` | Classify official SDK/API errors into retryable, permanent, and configuration failures. |
| `internal/integration/openmeter/client_test.go` | `httptest` request/response contract tests without a real OpenMeter. |
| `internal/integration/openmeter/errors_test.go` | Error classification tests. |
| `internal/integration/openmeter/poc_env_test.go` | Fail-closed opt-in environment parsing and readiness checks. |
| `internal/integration/openmeter/poc_fixture_test.go` | Create namespaced meters, features, customers, plan, subscription, and query helpers. |
| `internal/integration/openmeter/metering_contract_test.go` | COUNT, dedupe, tenant isolation, invalid-event, and same-ID/different-source contracts. |
| `internal/integration/openmeter/storage_contract_test.go` | LATEST upload/delete/out-of-order contract. |
| `internal/integration/openmeter/entitlement_contract_test.go` | `hasAccess`, derived balance/overage, tenant isolation, and concurrency experiment. |
| `internal/integration/openmeter/replay_contract_test.go` | Deterministic seed, outage, and replay phases. |
| `scripts/lib/openmeter-poc.ps1` | Pinned quickstart paths, Compose override generation, readiness, and evidence helpers. |
| `scripts/openmeter-poc.Tests.ps1` | Pester tests for pinning, path safety, redaction, and command construction. |
| `scripts/run-openmeter-poc.ps1` | Reproducible end-to-end runner and outage orchestration. |
| `docs/architecture/openmeter-shadow-metering-poc-report.md` | Immutable inputs, observed results, resource evidence, semantic gaps, and final three-way decision. |

---

### Task 1: Lock the dependency and non-production boundary

**Files:**

- Create: `tests/openmeter_poc_boundary_test.go`
- Create: `internal/integration/openmeter/doc.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Step 1: Add repository boundary guards**

Add these tests in package `tests`:

```go
func TestOpenMeterImportsStayInsideIsolatedAdapter(t *testing.T)
func TestOpenMeterPoCDoesNotEnterRuntimeConfigurationOrDeployments(t *testing.T)
```

The import test walks tracked `.go` files and permits the SDK import only below `internal/integration/openmeter`. The runtime test scans `cmd`, `config`, and `deployments` and rejects `OPENMETER_`, OpenMeter package imports, or OpenMeter service/image additions. Keep the scan scoped to those runtime/configuration paths so the PoC script and documentation remain allowed.

These are repository-policy guards rather than production behavior tests, so they do not need an artificial RED caused by grepping a dependency version. The exact dependency is verified through the Go module graph after installation.

**Step 2: Run the boundary guards on the clean runtime**

Run:

```powershell
go test ./tests -run OpenMeter -count=1
```

Expected: PASS because OpenMeter has not entered a production path.

**Step 3: Add the exact official SDK dependency**

Run:

```powershell
go get github.com/openmeterio/openmeter/api/v3/client@v1.0.0-beta.231
```

Create `doc.go` with package comment explaining that the package is a PoC-only adapter, is intentionally absent from bootstrap, and must not become a business-domain interface.

Do not run `go mod tidy` in this task: the first SDK import is introduced by Task 2, and tidying before that would correctly remove the otherwise unused dependency. Task 2 runs tidy after the import exists.

**Step 4: Run focused and default-safe tests**

Run:

```powershell
$module = go list -m -json github.com/openmeterio/openmeter/api/v3/client | ConvertFrom-Json
if ($module.Version -ne 'v1.0.0-beta.231') { throw "unexpected OpenMeter v3 SDK version: $($module.Version)" }
if (go list -m all | Select-String '^github.com/openmeterio/openmeter ') { throw 'legacy OpenMeter root module must not enter the dependency graph' }
go test ./tests -run OpenMeter -count=1
go test ./internal/integration/openmeter -count=1
```

Expected: module graph reports exactly `v1.0.0-beta.231`, legacy root module is absent, tests PASS, and no external service is contacted.

**Step 5: Commit**

```powershell
git add -- go.mod go.sum tests/openmeter_poc_boundary_test.go internal/integration/openmeter/doc.go
git commit -m "test: enforce isolated OpenMeter PoC boundary"
```

### Task 2: Define the stable usage-event contract

**Files:**

- Create: `internal/integration/openmeter/usage_event.go`
- Create: `internal/integration/openmeter/usage_event_test.go`

**Step 1: Write table-driven failing tests**

Cover these named cases:

```go
func TestBuildUsageEventCreatesStableCloudEvent(t *testing.T)
func TestBuildUsageEventMapsEachMetricToDistinctEventType(t *testing.T)
func TestBuildUsageEventRejectsInvalidFacts(t *testing.T)
func TestValidateUsageEventRejectsMetricTypeMismatch(t *testing.T)
func TestBuildUsageEventDataContainsOnlyAllowlistedFields(t *testing.T)
```

The stable-event test calls the constructor twice with the same fact and requires identical `Source`, `ID`, `Subject`, `Type`, `Time`, and `Data`. The invalid table must include empty tenant, unknown metric, empty source type/ID/revision, non-UTC/zero time, count quantity other than `1`, negative storage, fractional storage, exponent notation, and non-decimal storage.

**Step 2: Run the tests and confirm RED**

```powershell
go test ./internal/integration/openmeter -run 'UsageEvent|UsageFact' -count=1
```

Expected: compile failure because the contract types do not exist.

**Step 3: Implement the minimum contract**

Use these public shapes:

```go
type Metric string

const (
    MetricStudioDesignJobsSucceeded Metric = "studio_design_jobs_succeeded"
    MetricSheinDraftsSucceeded       Metric = "shein_drafts_succeeded"
    MetricStorageBytesCurrent        Metric = "storage_bytes_current"
)

type UsageFact struct {
    TenantID  string
    Metric    Metric
    Quantity  string
    SourceType string
    SourceID   string
    Revision   string
    OccurredAt time.Time
}

func BuildUsageEvent(fact UsageFact) (openmeter.EventInput, error)
func ValidateUsageEvent(event openmeter.EventInput) error
func SubjectForTenant(tenantID string) (string, error)
```

Implementation rules:

- Use source `task-processor/listingkit`.
- Use event type `listingkit.usage.` plus the validated catalog metric.
- Build a deterministic ID from tenant, metric, source type, source ID, and revision with unambiguous escaping; do not generate a UUID.
- Set `specversion=1.0`, `datacontenttype=application/json`, and use the official SDK's nullable helpers.
- Emit only `metric`, `quantity`, `source_type`, `source_id`, and `revision` in `Data`.
- Treat storage bytes as a non-negative base-10 integer string. Normalize leading zeroes but reject signs, fractions, and exponent notation.
- COUNT metrics require quantity `1`; their meter ignores the value but retaining it keeps one catalog contract.
- `ValidateUsageEvent` must recompute the event type from `data.metric` and reject a mismatch before ingest.

**Step 4: Run tests and formatting**

```powershell
gofmt -w internal/integration/openmeter/usage_event.go internal/integration/openmeter/usage_event_test.go
go mod tidy
go test ./internal/integration/openmeter -run 'UsageEvent|UsageFact' -count=1
```

Expected: PASS.

**Step 5: Commit**

```powershell
git add -- internal/integration/openmeter/usage_event.go internal/integration/openmeter/usage_event_test.go
git commit -m "feat: define OpenMeter usage event contract"
```

### Task 3: Wrap only the official v3 SDK surface

**Files:**

- Create: `internal/integration/openmeter/client.go`
- Create: `internal/integration/openmeter/client_test.go`
- Create: `internal/integration/openmeter/errors.go`
- Create: `internal/integration/openmeter/errors_test.go`

**Step 1: Write failing adapter tests with `httptest.Server`**

Test the official client's actual v3 paths and payloads through the wrapper:

```go
func TestClientIngestValidatesBeforeCallingSDK(t *testing.T)
func TestClientIngestUsesOfficialV3EventEndpoint(t *testing.T)
func TestClientQueryUsageFiltersBySubject(t *testing.T)
func TestClientListCustomerAccessReturnsFeatureAccess(t *testing.T)
func TestClassifyErrorUsesOfficialAPIError(t *testing.T)
```

The server must reject unexpected methods, paths, authorization headers, or JSON. The subject-query assertion must verify the SDK request contains a dimension filter equivalent to `subject == tenant:<id>`. Return numeric values as strings because SDK type `Numeric` is a string alias.

Error cases:

- HTTP 408, 429, and 5xx: retryable.
- connection reset, timeout, and temporary network errors: retryable.
- HTTP 400/404/409/422: permanent event or fixture rejection, preserving status and safe detail.
- HTTP 401/403: configuration/authentication failure, not a normal retry loop.
- malformed base URL and missing URL: configuration failure.

**Step 2: Run tests and confirm RED**

```powershell
go test ./internal/integration/openmeter -run 'Client|ClassifyError' -count=1
```

Expected: compile failure because the wrapper is absent.

**Step 3: Implement the thin wrapper**

Expose only:

```go
type Client struct { sdk *openmeter.Client }

type Config struct {
    BaseURL   string
    APIKey    string
    HTTPClient *http.Client
}

type FailureKind string

const (
    FailureRetryable FailureKind = "retryable"
    FailurePermanent FailureKind = "permanent"
    FailureConfiguration FailureKind = "configuration"
)

func NewClient(cfg Config) (*Client, error)
func (c *Client) Ingest(ctx context.Context, event openmeter.EventInput) error
func (c *Client) QueryUsage(ctx context.Context, meterID, subject string, from, to time.Time) (string, error)
func (c *Client) ListCustomerAccess(ctx context.Context, customerID string) ([]openmeter.EntitlementAccessResult, error)
func ClassifyError(err error) FailureKind
```

Construct the SDK with `openmeter.New`, `openmeter.WithHTTPClient`, and `openmeter.WithToken` only when a token exists. Use `openmeter.AsAPIError` for API status classification. `QueryUsage` must return `"0"` for an empty result, require exactly one row otherwise, and return its numeric string without converting storage bytes through `float64`.

Do not add fixture-creation methods to the production-shaped wrapper. Real PoC setup may call the official SDK directly from `_test.go` fixture code, keeping the adapter surface equal to the future decision boundary.

**Step 4: Run focused and boundary tests**

```powershell
gofmt -w internal/integration/openmeter/client.go internal/integration/openmeter/client_test.go internal/integration/openmeter/errors.go internal/integration/openmeter/errors_test.go
go test ./internal/integration/openmeter -run 'Client|ClassifyError' -count=1
go test ./tests -run OpenMeter -count=1
```

Expected: PASS.

**Step 5: Commit**

```powershell
git add -- internal/integration/openmeter/client.go internal/integration/openmeter/client_test.go internal/integration/openmeter/errors.go internal/integration/openmeter/errors_test.go
git commit -m "feat: add official OpenMeter v3 adapter"
```

### Task 4: Build fail-closed, namespaced real-environment fixtures

**Files:**

- Create: `internal/integration/openmeter/poc_env_test.go`
- Create: `internal/integration/openmeter/poc_fixture_test.go`

**Step 1: Write failing environment-gate tests**

Add pure tests that mutate environment with `t.Setenv`:

```go
func TestLoadPoCEnvironmentSkipsOnlyWhenNotEnabled(t *testing.T)
func TestLoadPoCEnvironmentFailsWhenEnabledWithoutURLOrRunID(t *testing.T)
func TestLoadPoCEnvironmentAcceptsOptionalAPIKey(t *testing.T)
func TestPoCNamesDeriveOnlyFromSanitizedRunID(t *testing.T)
```

Require:

- `OPENMETER_POC=1`
- `OPENMETER_POC_URL=http://127.0.0.1:48888/api/v3`
- non-empty `OPENMETER_POC_RUN_ID` matching `[a-z0-9][a-z0-9-]{0,39}`
- optional `OPENMETER_API_KEY`
- optional `OPENMETER_POC_PHASE`, restricted to `contract`, `seed`, `unavailable`, or `replay`

Do not use `t.Skip` after opt-in.

**Step 2: Run and confirm RED**

```powershell
go test ./internal/integration/openmeter -run 'PoCEnvironment|PoCNames' -count=1
```

**Step 3: Implement environment parsing and fixture creation**

The real-test helper must:

1. Create three meters with namespaced keys and distinct `EventType` values.
2. Use `MeterAggregationCount` for Studio/SHEIN and `MeterAggregationLatest` with `ValueProperty="$.quantity"` for storage.
3. Create three features referencing those meter IDs.
4. Create two customers, each with one unique subject in `CustomerUsageAttribution.SubjectKeys`.
5. Create one USD zero-price plan with `BillingCadence="P1M"`, a single indefinite phase, and three metered entitlements: 5 Studio, 3 SHEIN, and 10 MiB storage.
6. Publish the plan and subscribe both customers using the official v3 SDK.

Construct the tagged unions only with official helpers such as `PriceFromPriceFree` and `RateCardEntitlementFromRateCardMeteredEntitlement`. Use exact fixture keys from sanitized `OPENMETER_POC_RUN_ID`; if creation returns conflict, fetch and validate the existing resource rather than silently accepting incompatible configuration.

Add bounded polling helpers for asynchronous meter visibility. Poll every 250 ms for at most 30 seconds, report the last safe result on timeout, and never use an unbounded sleep.

**Step 4: Run default-safe tests**

```powershell
Remove-Item Env:OPENMETER_POC -ErrorAction SilentlyContinue
go test ./internal/integration/openmeter -count=1
```

Expected: pure tests pass and real contract setup reports SKIP.

**Step 5: Commit**

```powershell
git add -- internal/integration/openmeter/poc_env_test.go internal/integration/openmeter/poc_fixture_test.go
git commit -m "test: add OpenMeter PoC fixture boundary"
```

### Task 5: Prove metering, deduplication, isolation, and LATEST semantics

**Files:**

- Create: `internal/integration/openmeter/metering_contract_test.go`
- Create: `internal/integration/openmeter/storage_contract_test.go`

**Step 1: Write the real contract tests before the runner**

Add:

```go
func TestPoCCountMetersAggregateCommittedSuccesses(t *testing.T)
func TestPoCDuplicateSourceAndIDCountsOnce(t *testing.T)
func TestPoCSameIDDifferentSourceCountsSeparately(t *testing.T)
func TestPoCTenantSubjectsRemainIsolated(t *testing.T)
func TestPoCInvalidEventsNeverReachOpenMeter(t *testing.T)
func TestPoCStorageLatestSupportsIncreaseAndDecrease(t *testing.T)
func TestPoCStorageLatestUsesBusinessTimeForOutOfOrderEvents(t *testing.T)
```

Each test must use a unique suffix under the run namespace and explicit UTC windows. Assert exact decimal strings after bounded polling. For duplicate replay, send the identical official SDK event 10 times and require a count of `1`. For same ID/different source, preserve ID but mutate CloudEvent `Source` and require `2`; document that production retries must not change source.

The invalid-event table must prove no HTTP request is made for unknown metric, empty tenant, negative/non-decimal storage, and mutated event type/data.metric mismatch.

The LATEST out-of-order test ingests a newer timestamp/value first and an older timestamp/value second. The final result must retain the newer business-time value. A failure is a recorded semantic failure, not a test relaxation.

**Step 2: Run default mode and confirm safe SKIP**

```powershell
Remove-Item Env:OPENMETER_POC -ErrorAction SilentlyContinue
go test ./internal/integration/openmeter -run 'TestPoC(Count|Duplicate|SameID|Tenant|Invalid|Storage)' -count=1 -v
```

Expected: all real tests are reported as SKIP because the PoC is not enabled; no connection attempt occurs.

**Step 3: Review the fixture wiring without weakening assertions**

Compile and run pure tests:

```powershell
go test ./internal/integration/openmeter -run 'UsageEvent|Client|PoCEnvironment|PoCNames' -count=1
```

If SDK request shapes do not compile, adjust only to the pinned SDK's generated types. Do not replace official union constructors or introduce map-based HTTP payloads.

**Step 4: Commit**

```powershell
git add -- internal/integration/openmeter/metering_contract_test.go internal/integration/openmeter/storage_contract_test.go
git commit -m "test: define OpenMeter metering contracts"
```

### Task 6: Prove entitlement behavior and expose the concurrency limit

**Files:**

- Create: `internal/integration/openmeter/entitlement_contract_test.go`

**Step 1: Add entitlement contract tests**

```go
func TestPoCEntitlementAccessTracksUsageThresholds(t *testing.T)
func TestPoCEntitlementsRemainTenantIsolated(t *testing.T)
func TestPoCStorageAccessRecoversAfterUsageDrops(t *testing.T)
func TestPoCConcurrentAccessCheckDoesNotPromiseAtomicReservation(t *testing.T)
```

For each threshold, query meter usage and call `ListCustomerAccess`. Calculate evidence-only values with integer decimal arithmetic:

```text
balance = limit - usage
overage = max(usage - limit, 0)
```

Assert `hasAccess` against the configured hard-limit behavior at zero, partial, exact-limit, and above-limit usage. Clearly label balance/overage as locally derived because the pinned v3 access API exposes only `hasAccess`, feature key, type, and static config.

For storage, ingest 8 MiB then 3 MiB as absolute snapshots and prove access/balance recover after the lower LATEST value becomes current.

For concurrency, leave one unit and release 20 goroutines through a start barrier. Each goroutine queries access before ingesting a unique event. Record how many saw access and the final usage. The test passes only when it produces a reproducible observation and writes it to `t.Log`; it must not assert that OpenMeter provides an atomic business reservation. The report later decides whether local reservation/commit/release is mandatory.

**Step 2: Run default-safe mode**

```powershell
Remove-Item Env:OPENMETER_POC -ErrorAction SilentlyContinue
go test ./internal/integration/openmeter -run 'TestPoC(Entitlement|Entitlements|StorageAccess|Concurrent)' -count=1 -v
```

Expected: SKIP with no external access.

**Step 3: Commit**

```powershell
git add -- internal/integration/openmeter/entitlement_contract_test.go
git commit -m "test: define OpenMeter entitlement experiments"
```

### Task 7: Add deterministic outage/replay orchestration

**Files:**

- Create: `internal/integration/openmeter/replay_contract_test.go`
- Create: `scripts/lib/openmeter-poc.ps1`
- Create: `scripts/openmeter-poc.Tests.ps1`
- Create: `scripts/run-openmeter-poc.ps1`

**Step 1: Write failing replay-phase tests**

Add:

```go
func TestPoCReplaySeed(t *testing.T)
func TestPoCUnavailableClassifiesFailureAsRetryable(t *testing.T)
func TestPoCReplayAfterRecoveryConvergesExactly(t *testing.T)
```

Use deterministic event IDs derived from run ID plus fixed logical facts:

- `seed`: ingest three success facts and wait for exact count `3`.
- `unavailable`: ingest a fourth fact while the API service is stopped and require a retryable error; do not change its event identity.
- `replay`: resend all four facts, resend the fourth ten additional times, and require exact final count `4`.

Each test runs only when its matching `OPENMETER_POC_PHASE` is selected.

**Step 2: Write failing Pester tests for the runner library**

`scripts/openmeter-poc.Tests.ps1` must verify:

- generated override pins `openmeter`, `sink-worker`, `balance-worker`, `notification-service`, `billing-worker`, and `openmeter-jobs` to `ghcr.io/openmeterio/openmeter:v1.0.0-beta.232`;
- Compose project name is exactly `task-processor-openmeter-poc`;
- checkout/evidence paths resolve below `<repo>/.local/openmeter-poc`;
- a runner execution against controlled command fakes invokes `docker compose down` without `-v` and never invokes volume removal or recursive deletion;
- captured command/log output redacts the injected `OPENMETER_API_KEY` value;
- failure of clone, Compose health, digest resolution, a Go phase, or resource capture returns nonzero.

Run and confirm RED:

```powershell
Invoke-Pester ./scripts/openmeter-poc.Tests.ps1
```

**Step 3: Implement the runner library and entrypoint**

`scripts/run-openmeter-poc.ps1` parameters:

```powershell
param(
    [string]$RunId = (Get-Date -Format 'yyyyMMdd-HHmmss'),
    [string]$ApiKey,
    [switch]$KeepEnvironment
)
```

The runner must execute this exact lifecycle:

1. Resolve repository root and validate that all generated paths remain under `.local/openmeter-poc`.
2. Verify `git`, `go`, `docker`, and `docker compose` availability.
3. Shallow-clone the official OpenMeter repository at tag `v1.0.0-beta.232`, or validate an existing checkout's origin and exact tag.
4. Generate an ignored Compose override that pins all six OpenMeter-owned images; use the upstream `quickstart/docker-compose.yaml` unchanged as the base.
5. Run `docker compose config` into the run evidence directory and reject any OpenMeter-owned image still using `latest`.
6. Start with `docker compose up -d --wait`; verify `http://127.0.0.1:48888/api/v3` before testing.
7. Capture service list, image tags, `RepoDigests`, and `docker stats --no-stream` before load.
8. Run default-safe unit/boundary tests.
9. Run real `contract` tests.
10. Run replay `seed`; stop only the `openmeter` API service; run `unavailable`; restart and wait healthy; run `replay`.
11. Capture after-load stats and exact Go test logs.
12. Run `docker compose down` without `-v` unless `-KeepEnvironment` is set. Preserve the evidence directory either way.

Set environment only for each child test process:

```powershell
$env:OPENMETER_POC = '1'
$env:OPENMETER_POC_URL = 'http://127.0.0.1:48888/api/v3'
$env:OPENMETER_POC_RUN_ID = $RunId
$env:OPENMETER_POC_PHASE = $phase
```

Do not print `$ApiKey`; pass it through `OPENMETER_API_KEY` and restore/remove all variables in `finally`.

**Step 4: Run script unit tests and default Go tests**

```powershell
Invoke-Pester ./scripts/openmeter-poc.Tests.ps1
Remove-Item Env:OPENMETER_POC -ErrorAction SilentlyContinue
go test ./internal/integration/openmeter ./tests -run 'OpenMeter|UsageEvent|Client|PoC' -count=1
```

Expected: Pester PASS; Go pure/boundary tests PASS; real tests SKIP.

**Step 5: Commit**

```powershell
git add -- internal/integration/openmeter/replay_contract_test.go scripts/lib/openmeter-poc.ps1 scripts/openmeter-poc.Tests.ps1 scripts/run-openmeter-poc.ps1
git commit -m "test: orchestrate OpenMeter outage replay PoC"
```

### Task 8: Execute the PoC and write the decision report

**Files:**

- Create: `docs/architecture/openmeter-shadow-metering-poc-report.md`
- Modify contract/runner files from Tasks 4–7 only if real evidence exposes a genuine contract or SDK compatibility defect.

**Step 1: Establish a clean, exact baseline**

```powershell
git status --short
git rev-parse HEAD
docker version
docker compose version
go version
```

Expected: only intended work is present. If unrelated user changes exist, preserve them and keep them out of every stage/commit.

**Step 2: Run the complete pinned PoC**

```powershell
$runId = 'om-' + (Get-Date -Format 'yyyyMMdd-HHmmss')
./scripts/run-openmeter-poc.ps1 -RunId $runId
```

Expected: exit 0 only when unit, boundary, metering, LATEST, entitlement, outage, and replay phases all completed and evidence capture succeeded.

If a core semantic test fails, keep the assertion. Use systematic debugging to distinguish adapter error, SDK/server version incompatibility, eventual-consistency timing, and genuine OpenMeter behavior. A genuine dedupe, tenant-isolation, replay, or LATEST failure must remain visible in the report and influence the final decision.

**Step 3: Write the report from captured evidence**

The report must contain complete, non-placeholder sections:

1. Scope and exact repository SHA.
2. OpenMeter tag, upstream Git SHA, v3 SDK version, image tags, and resolved digests.
3. Rendered service inventory and local port exposure.
4. Test matrix with command, pass/fail/skip counts, expected value, actual value, and evidence filename.
5. COUNT, dedupe, source identity, tenant isolation, invalid-event, LATEST, outage, and replay findings.
6. Entitlement table at zero/partial/limit/over-limit, separating API-native `hasAccess` from derived balance/overage.
7. Concurrency observations and whether local reservation/commit/release is required.
8. Before/after resource table for every quickstart component.
9. SDK/API gaps, including the service `beta.232` versus SDK `beta.231` compatibility result and absence of native v3 balance/overage fields.
10. Mapping from failure modes to future PAY-041 outbox retry, dead-letter, and manual adjustment responsibilities.
11. Exactly one final decision: adopt for metering and entitlement, adopt for metering only, or reject.
12. Explicit statement that no production integration, payment, billing, deployment, or data migration was performed.

Do not commit raw logs, Compose secrets, API keys, or the `.local` evidence directory.

**Step 4: Run final verification before the report commit**

```powershell
Invoke-Pester ./scripts/openmeter-poc.Tests.ps1
Remove-Item Env:OPENMETER_POC -ErrorAction SilentlyContinue
go test ./internal/integration/openmeter -count=1
go test ./tests -run OpenMeter -count=1
./scripts/test-all.ps1 -count=1
git diff --check
git status --short
```

Expected:

- Pester PASS.
- Pure/default-safe OpenMeter tests PASS and real tests SKIP.
- Repository-wide Go suite PASS.
- No whitespace errors.
- Only the report or intentional evidence-driven fixes are uncommitted.

**Step 5: Self-review against the design**

Before committing, verify every design gate has an evidence row and confirm:

- no `TODO`, `TBD`, placeholder digest, or unfilled result remains in the report;
- every metric uses a distinct CloudEvent type;
- all quantities remain decimal strings end to end;
- `hasAccess` is not mislabeled as atomic reservation;
- balance/overage are labeled derived, not native v3 fields;
- quickstart `latest` was overridden and the actual digest was recorded;
- no test failure was converted into skip after opt-in;
- no production package outside the isolated adapter imports OpenMeter;
- no production config, deployment, database, or billing/payment path changed.

**Step 6: Commit the evidence-backed decision**

```powershell
git add -- docs/architecture/openmeter-shadow-metering-poc-report.md
git commit -m "docs: record OpenMeter metering PoC decision"
```

If real evidence required adapter/test corrections, stage those exact files with the report and explain the causal correction in the commit body; do not include unrelated changes.

## Completion Boundary

This plan is complete only when the report contains one of the three allowed decisions backed by a reproducible run. A successful PoC does not authorize PAY-041, production shadow writes, deployment, billing, payment, migration, traffic switching, or deletion of existing counters. The next step after a positive result is a separate PAY-041 design for transactional usage event/outbox ownership; the next step after a rejected result is a separately approved Flexprice PoC using the same product contracts and evidence gates.
