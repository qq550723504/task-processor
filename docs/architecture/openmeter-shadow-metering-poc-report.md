# OpenMeter shadow-metering PoC decision

Date: 2026-08-13

Final evidence run: `om-20260813-224107`

Decision source SHA: `50ed0a06bd3e4248836011ba18296b283b40bdf9`

## Decision

**adopt for metering and entitlement**

The pinned local service satisfied the required COUNT, CloudEvent identity,
tenant-isolation, invalid-event, LATEST, outage/replay, and metered-entitlement
contracts. The access API is suitable for ordinary entitlement checks. It is
not an atomic quota reservation primitive: a later PAY-041 design must keep
transactional reservation/commit/release local to ListingKit for hard concurrent
admission.

This decision records technical fit only. It does not authorize production
integration, shadow writes, traffic, or a billing cutover.

## Scope and evidence provenance

The PoC ran only against an isolated Docker Compose project on the development
machine. The official upstream checkout and all raw evidence are under the
ignored `.local/openmeter-poc` directory and are not committed.

| Item | Exact value | Evidence |
| --- | --- | --- |
| task-processor source | `50ed0a06bd3e4248836011ba18296b283b40bdf9` | `task-processor-git-sha.txt`, `runner.log` |
| Final run | `om-20260813-224107`, exit 0 | `runner.log` |
| OpenMeter tag | `v1.0.0-beta.232` | `runner.log` exact-tag check |
| OpenMeter upstream source | `887e0cac903ccd06e74d61ed23c651651d10c7a9` | `upstream-git-sha.txt`, image OCI revision label |
| OpenMeter Go v3 client | `github.com/openmeterio/openmeter/api/v3/client v1.0.0-beta.231` | repository `go.mod` |
| Go | `go1.26.5 windows/amd64` | `runner.log` |
| Docker Engine | `29.7.2` | `runner.log` |
| Docker Compose | `v5.3.1` | `runner.log` |
| Evidence root | `.local/openmeter-poc/evidence/om-20260813-224107/` | ignored local directory |

The runner verified that the upstream remote was
`https://github.com/openmeterio/openmeter.git`, HEAD exactly matched the pinned
tag, and `quickstart/docker-compose.yaml` was unmodified relative to that tag.

## Images and immutable digests

The local override replaced the quickstart's floating OpenMeter image for every
OpenMeter-owned service. The rendered model contained no OpenMeter-owned
`latest` reference. The other quickstart images were already digest-pinned.

| Services | Rendered image | Resolved repository digest |
| --- | --- | --- |
| `openmeter`, `openmeter-jobs`, `balance-worker`, `billing-worker`, `notification-service`, `sink-worker` | `ghcr.io/openmeterio/openmeter:v1.0.0-beta.232` | `ghcr.io/openmeterio/openmeter@sha256:dd957045cf2fab2e93c69e8f838e2ed5a05d94f80733c62ba2dd30753850f7bc` |
| `clickhouse` | `clickhouse/clickhouse-server:25.12.3-alpine@sha256:74da41cd61db84f652c6364fd30d59e19b7276d34f7c82515f5f0e70d6f325da` | `clickhouse/clickhouse-server@sha256:74da41cd61db84f652c6364fd30d59e19b7276d34f7c82515f5f0e70d6f325da` |
| `kafka` | `confluentinc/cp-kafka:8.0.3@sha256:db5eac24a1d15a1689fa89642ef97a7e3bc4f55ad5159a07b213c4dc7d0114e3` | `confluentinc/cp-kafka@sha256:db5eac24a1d15a1689fa89642ef97a7e3bc4f55ad5159a07b213c4dc7d0114e3` |
| `postgres` | `postgres:14.20-alpine3.23@sha256:14f02666642586a64d6fae8ef42d479fd76456a77c73ae8a626b8fe323b76d22` | `postgres@sha256:14f02666642586a64d6fae8ef42d479fd76456a77c73ae8a626b8fe323b76d22` |
| `redis` | `redis:7.4.7@sha256:c6e4a82ce60b829a72a21cc4ad4c1c274fa6962eb0d6ac698e8157cbc3fe16a4` | `redis@sha256:c6e4a82ce60b829a72a21cc4ad4c1c274fa6962eb0d6ac698e8157cbc3fe16a4` |
| `svix` | `svix/svix-server:v1.84.1@sha256:35dc9d4884073272e55f8e2f60a7a37983e535b7e7994db56a99c87cdf9b1a59` | `svix/svix-server@sha256:35dc9d4884073272e55f8e2f60a7a37983e535b7e7994db56a99c87cdf9b1a59` |

Evidence: `image-tags.txt`, `openmeter-repo-digests.json`,
`compose.rendered.json`, and `compose-services.jsonl`.

## Rendered service inventory and port exposure

All published ports were bound to `127.0.0.1`; none was exposed on all host
interfaces. Ports shown without a host mapping were container/network-only.

| Service | Captured state | Health | Local exposure |
| --- | --- | --- | --- |
| `balance-worker` | running | healthy | `127.0.0.1:40001 -> 10000/tcp` |
| `billing-worker` | running | healthy | `127.0.0.1:40003 -> 10000/tcp` |
| `clickhouse` | running | healthy | network-only `8123`, `9000`, `9009/tcp` |
| `kafka` | running | healthy | network-only `9092/tcp` |
| `notification-service` | running | healthy | `127.0.0.1:40002 -> 10000/tcp` |
| `openmeter` | running | healthy | `127.0.0.1:48888 -> 8888/tcp` |
| `openmeter-jobs` | running | no Compose healthcheck | none |
| `postgres` | running | healthy | network-only `5432/tcp` |
| `redis` | running | healthy | network-only `6379/tcp` |
| `sink-worker` | running | healthy | `127.0.0.1:40000 -> 10000/tcp` |
| `svix` | running | healthy | network-only `8071/tcp` |

The API readiness probe used
`http://127.0.0.1:48888/api/v1/debug/metrics`; the SDK base remained
`http://127.0.0.1:48888/api/v3`. Evidence: `compose.rendered.json`,
`compose-services.jsonl`, and `runner.log`.

## Test matrix

Counts below are terminal Go JSON events with a non-empty test name; parent
tests and subtests are counted separately. Phase-specific replay tests are
expected to skip during the broad default/contract commands and are then each
required to pass in their selected phase. No opted-in semantic failure was
converted to a skip.

| Command | Pass / fail / skip | Expected value | Actual value | Evidence |
| --- | ---: | --- | --- | --- |
| `go test -json ./internal/integration/openmeter ./tests -run OpenMeter\|UsageEvent\|Client\|PoC -count=1` | 82 / 0 / 15 | Pure and boundary tests pass; all 15 real-service tests remain default-safe | All pure/boundary tests passed and exactly 15 real-service tests skipped with `OPENMETER_POC` unset | `go-test-default.log` |
| `go test -json ./internal/integration/openmeter -run ^TestPoC -count=1 -v` | 29 / 0 / 3 | Fixture, COUNT, identity, isolation, invalid input, LATEST, entitlement, and concurrency pass; only three other replay phases skip | All contract tests and subtests passed; only seed/unavailable/replay phase tests skipped | `go-test-contract.log` |
| `go test -json ./internal/integration/openmeter -run ^TestPoCReplaySeed$ -count=1 -v` | 1 / 0 / 0 | Seed three unique successes and observe aggregate `3` | Exact aggregate became `3` | `go-test-replay-seed.log` |
| `go test -json ./internal/integration/openmeter -run ^TestPoCUnavailableClassifiesFailureAsRetryable$ -count=1 -v` | 1 / 0 / 0 | Stopped API yields a retryable error without changing event identity | Windows connection refusal was classified retryable; identity remained unchanged | `go-test-replay-unavailable.log` |
| `go test -json ./internal/integration/openmeter -run ^TestPoCReplayAfterRecoveryConvergesExactly$ -count=1 -v` | 1 / 0 / 0 | After restart, four unique events plus ten replays of event four converge to `4` for four samples | Exact aggregate was `4` for four consecutive samples | `go-test-replay-recovery.log` |

The orchestrated run also required successful upstream verification, rendered
Compose validation, image digest resolution, health verification, before/after
resource capture, and final cleanup. `runner.log` records exit 0 for every
required command and ends with `OpenMeter PoC run completed` followed by a
successful `docker compose ... down` without `-v`.

## Metering, identity, isolation, and LATEST findings

| Contract | Expected | Actual |
| --- | --- | --- |
| COUNT committed successes | Studio `3`; SHEIN `2` | Exact aggregates `3` and `2` |
| Duplicate delivery | Ten submissions with identical `source + id` count once | Exact aggregate `1` |
| Source identity boundary | Same `id` under two sources is two CloudEvents | Exact aggregate `2`; production adapter continues to forbid source mutation |
| Tenant isolation | Tenant A `2`, tenant B `4` on one meter | Exact independent aggregates `2` and `4` |
| Invalid events | Six invalid shapes fail before transport | Unknown metric, empty tenant, negative/non-decimal storage, event-type mismatch, and data-metric mismatch each made zero HTTP requests |
| Storage increase/decrease | LATEST follows `100 -> 200 -> 50` | Each value became visible in order, including the decrease to `50` |
| Out-of-order storage | Newer business-time `900`, then stored older `100`, remains `900` | Older event was visibly stored without validation errors; query remained `900` for four samples |
| Outage/replay | Seed `3`; stopped API retryable; stable event replay reaches exactly `4` | All three separately selected phases passed |

Each product metric has a distinct CloudEvent type:

- `studio_design_jobs_succeeded` -> `listingkit.usage.studio_design_jobs_succeeded`
- `shein_drafts_succeeded` -> `listingkit.usage.shein_drafts_succeeded`
- `storage_bytes_current` -> `listingkit.usage.storage_bytes_current`

Usage quantities remained base-10 strings through `UsageFact`, CloudEvent data,
and query results. The exercised values included `"1"`, `"100"`, `"200"`,
`"50"`, and `"900"`. Plan limits are configuration values in the generated
SDK schema, not usage-event quantities. Evidence: `go-test-contract.log` and the
contract assertions represented by that log.

## Entitlement evidence

The studio feature had a hard limit of `5`. `hasAccess` is the native v3 access
result. Balance and overage below were calculated locally from the configured
limit and metered usage solely to cross-check that result; they are not fields
returned by the native v3 access response.

| State | Meter usage | Native `hasAccess` | Derived balance | Derived overage | Result |
| --- | ---: | --- | ---: | ---: | --- |
| zero | `0` | `true` | `5` | `0` | pass |
| partial | `2` | `true` | `3` | `0` | pass |
| exact limit | `5` | `false` | `0` | `0` | pass |
| above limit | `6` | `false` | `-1` | `1` | pass |

Entitlement tenancy also remained isolated. On the SHEIN feature with limit
`3`, customer A at usage `4` stayed denied while customer B moved independently
from usage `0`/access `true` to usage `2`/access `true`. A later recheck still
reported customer A at usage `4`/access `false`.

LATEST-backed entitlement recovered when usage fell: storage usage
`12,582,912` bytes against a `10,485,760` byte limit produced native access
`false` and derived overage `2,097,152`; the later `3,145,728` byte snapshot
restored native access `true` with derived balance `7,340,032`.

Evidence: `go-test-contract.log`.

## Concurrency and hard-quota boundary

The test seeded studio usage at `4`, leaving one unit before the hard limit,
then released 20 workers to query access and submit 20 distinct events. In the
final run, **10 of 20** workers observed native access before ingest, while the
exact final usage converged to **24**.

This is an observation of eventually updated access state, not an atomic
reservation guarantee. OpenMeter `hasAccess` cannot by itself bind the check
and the business transaction or guarantee that only one concurrent caller
consumes the last unit. PAY-041 therefore must define a database-backed local
reservation/commit/release lifecycle, in the same transactional ownership
boundary as the business success and usage outbox. Process mutexes and client
locks are not sufficient.

Evidence: `go-test-contract.log`.

## Resource observations

These are two `docker stats --no-stream` snapshots from a single development
run, not sizing claims or a production benchmark. OpenMeter was deliberately
stopped and recreated between snapshots, so its network counters reset. Every
quickstart component is included.

| Component | CPU before -> after | Memory before -> after | Network I/O before -> after | Block I/O before -> after | PIDs before -> after |
| --- | --- | --- | --- | --- | ---: |
| `balance-worker` | `0.54% -> 1.31%` | `36.3MiB -> 41.59MiB` | `45.8kB/39.7kB -> 572kB/314kB` | `0B/0B -> 0B/0B` | `17 -> 18` |
| `billing-worker` | `0.13% -> 0.19%` | `34.83MiB -> 36.06MiB` | `23.8kB/21.5kB -> 218kB/128kB` | `0B/0B -> 0B/0B` | `15 -> 15` |
| `clickhouse` | `10.77% -> 23.22%` | `238MiB -> 373.6MiB` | `16.7kB/14.1kB -> 505kB/1.98MB` | `0B/639kB -> 0B/20MB` | `702 -> 733` |
| `kafka` | `10.48% -> 15.03%` | `397.1MiB -> 422.8MiB` | `79.5kB/106kB -> 808kB/1.79MB` | `0B/909kB -> 0B/2.1MB` | `118 -> 118` |
| `notification-service` | `0.13% -> 0.29%` | `33.68MiB -> 36.07MiB` | `20.8kB/18.6kB -> 183kB/80.4kB` | `0B/0B -> 0B/0B` | `15 -> 15` |
| `openmeter` | `0.42% -> 0.50%` | `87.81MiB -> 80.04MiB` | `327kB/782kB -> 145kB/107kB` | `0B/0B -> 0B/0B` | `23 -> 25` |
| `openmeter-jobs` | `0.52% -> 0.33%` | `35.23MiB -> 34.47MiB` | `10kB/7.8kB -> 35.7kB/21.4kB` | `0B/0B -> 0B/0B` | `23 -> 23` |
| `postgres` | `0.02% -> 0.05%` | `63.52MiB -> 78.75MiB` | `834kB/348kB -> 2.8MB/2.38MB` | `0B/95.6MB -> 0B/105MB` | `13 -> 18` |
| `redis` | `0.76% -> 1.59%` | `3.895MiB -> 4.652MiB` | `81.7kB/39.3kB -> 354kB/163kB` | `0B/0B -> 0B/0B` | `6 -> 6` |
| `sink-worker` | `0.56% -> 2.39%` | `34.11MiB -> 38.11MiB` | `35.2kB/13.3kB -> 1.08MB/332kB` | `0B/0B -> 0B/0B` | `22 -> 22` |
| `svix` | `0.53% -> 0.52%` | `14.29MiB -> 14.11MiB` | `85.1kB/139kB -> 203kB/366kB` | `0B/0B -> 0B/0B` | `14 -> 14` |

Each memory snapshot used a `31.25GiB` Docker limit. Evidence:
`docker-stats-before.jsonl` and `docker-stats-after.jsonl`.

## SDK/API compatibility and gaps

- The service was `v1.0.0-beta.232` while the generated Go v3 client was
  `v1.0.0-beta.231`. The final run exercised event ingest/list, meter and usage
  query, feature, customer/subject attribution, plan publish/reuse,
  subscription, and entitlement access successfully. This one-beta-version
  skew was compatible for the tested contract surface; it is not a promise
  about untested endpoints.
- Real execution exposed service contract constraints that the fixture had to
  honor: resource keys match `^[a-z0-9]+(?:_[a-z0-9]+)*$`; a rate-card key must
  equal its feature key; omitted entitlement `usage_period` is returned as the
  billing cadence (`P1M`); plan keys are versioned and an existing applicable
  plan must be reused; metered event subjects must be attributed to a customer.
- The diagnostic beta.232 plan response identified
  `extensions.validationErrors[0].code=rate_card_key_feature_key_mismatch` on
  the rate-card `key`. The SDK error presentation did not surface that nested
  detail in the original runner failure, so diagnosis used the same local
  service directly; production code still uses the official generated SDK.
- Native v3 entitlement access returns `hasAccess` but no balance or overage
  field. UI display or reconciliation must derive those values from usage and
  configured limits, label them derived, and retain exact decimal handling.
- The quickstart comprises OpenMeter plus ClickHouse, Kafka, Postgres, Redis,
  and Svix. Its observed local footprint and topology must not be treated as a
  production deployment design.

### Evidence-driven corrections

No semantic assertion was weakened. Failed diagnostic runs were retained in
the ignored evidence root and led to narrow test/fixture/runner repairs before
the clean final run.

| Diagnostic run | Root cause | Causal correction |
| --- | --- | --- |
| `om-20260813-215832` | PowerShell closure scope hid the default health function | Invoke the built-in probe directly when no injected probe is supplied; add runner test |
| `om-20260813-220557` | `/api/v3` is an SDK base, not a health endpoint | Separate v3 API base from `/api/v1/debug/metrics` readiness URL |
| `om-20260813-220837` | Empty phase accidentally enabled real tests; fixture keys violated beta.232 syntax | Treat only non-empty phases as opt-in; generate underscore-delimited keys and test the service regex/length |
| `om-20260813-221418` / `om-20260813-221540` | Plan returned HTTP 400 because each rate-card key differed from its feature key | Preserve the service validation and make the keys identical |
| `om-20260813-221933` | Server defaulted omitted entitlement usage period to `P1M` | Validate an omitted request value against the billing cadence default |
| `om-20260813-222424` | Recreating a versioned plan key produced a new plan ID and subscription mismatch | Look up and validate the exact existing plan before creating a draft |
| `om-20260813-222820` | Out-of-order LATEST event used a synthetic subject without customer attribution | Use the fixture's second attributed tenant while preserving the out-of-order assertion |
| `om-20260813-223143` | A real Windows refusal is `WSAECONNREFUSED`, not POSIX `ECONNREFUSED` | Add platform-specific OSS `x/sys/windows` refusal/reset classification plus a real closed-loopback test |

The causal corrections are committed in `50ed0a06bd3e4248836011ba18296b283b40bdf9`.
The final evidence run was made only after that commit, so its recorded source
SHA is reproducible.

## Final verification status

Task 8's focused gates passed after the report was written:

| Verification | Result |
| --- | --- |
| `Invoke-Pester ./scripts/openmeter-poc.Tests.ps1` | pass: 15, fail: 0, skip: 0 |
| `Invoke-Pester ./scripts/test-all.Tests.ps1` | pass: 2, fail: 0, skip: 0 |
| `Remove-Item Env:OPENMETER_POC ...; go test ./internal/integration/openmeter -count=1` | pass |
| `go test ./tests -run OpenMeter -count=1` | pass |
| Root module: `go test -count=1 ./cmd/... ./internal/... ./tests/...` | pass |
| Tools module: `go test -count=1 ./...` from `tools` | pass |
| Debug module: `go test -count=1 ./...` from `hack/debug` | pass |
| `./scripts/test-all.ps1 -count=1` | pass: root, tools, and debug modules; exit 0 in 95.7 seconds |
| `git diff --check` | pass |

The original `scripts/test-all.ps1` passed the nested `tools` and `hack/debug`
patterns to the root module. Go therefore rejected both package patterns before
their tests ran. An explicitly authorized, behavior-tested correction now runs
the root, tools, and debug modules in their own directories, forwards caller
arguments, preserves visible output, and returns the first failing nested exit
code. Pester RED showed one root call rather than three and showed that a
configured tools failure returned `0` instead of `23`; GREEN is 2/2. The
correction is commit `0ed04069d`.

The corrected command then exposed a separate, pre-existing `hack/debug`
module-integrity failure:

```text
go: updates to go.mod needed; to update it:
        go mod tidy
```

A read-only `go mod tidy -diff` established the dependency set. After explicit
user authorization, the persisted Git diff added 24 indirect requirements and
removed none in `go.mod`; it added 75 checksums and removed none in `go.sum`.
The debug source files directly import only `playwright-go` and `logrus`; both
are already direct requirements, alongside the root `task-processor` module.
There is no missing direct source import. The proposed indirect additions are
the transitive closure exposed by the debug tools' imports of root packages.
The metadata-only repair is commit `1d809e7f0`. A subsequent `go mod tidy -diff`
was empty, the debug module passed, and the complete module-aware harness passed
all three modules with exit 0. The original blocker was already documented in
`docs/refactoring/code-health-decisions.md` before Task 8.

The architecture-document guard also required the fixed-name report to be
listed in `docs/architecture/README.md`; the authorized minimal index entry was
added, after which the complete root-module suite passed. Neither harness or
documentation-index change affects the successful Docker evidence run or its
source SHA.

## PAY-041 ownership mapping

| Failure mode or observation | Future outbox/retry responsibility | Dead-letter/manual-adjustment responsibility |
| --- | --- | --- |
| Local contract validation fails | Reject before transport and do not mark delivered | Record the business validation cause; repair producer data rather than retry unchanged poison input |
| Refusal, reset, deadline, HTTP 408/429, or server 5xx | Keep row pending; retry with backoff and the immutable original CloudEvent identity | Escalate only after policy thresholds; never generate a replacement identity |
| HTTP 401/403 configuration failure | Pause blind dispatch and alert on endpoint/credential configuration | Quarantine until configuration is corrected; preserve the original row |
| Other permanent 4xx, including unbound subject or resource contract error | Do not hot-loop; preserve response classification and validation detail | Dead-letter for attribution/config repair, then use an explicitly audited redrive |
| Response is ambiguous after possible acceptance | Replay the exact same `source + id`; mark delivered only after an accepted response | Reconciliation owns unresolved ambiguity; do not count a regenerated event |
| Same `id` is emitted under a different source | Treat as a producer identity violation because OpenMeter counts it separately | Audit and manually reconcile the duplicate; the dispatcher must never mutate source |
| Business correction after a delivered COUNT fact | Emit only a separately designed, auditable correction supported by the product contract | PAY-041/PAY-044 own human approval and counter/OpenMeter reconciliation; never rewrite the original identity |
| LATEST storage correction | Emit a new snapshot with authoritative business time and stable unique identity | Investigate stale/source data; manual adjustment owns any disputed snapshot chronology |
| Concurrent hard-quota admission | Reserve locally in the business database transaction, commit on success, release on failure, and enqueue usage atomically | Reconcile leaked/expired reservations; native `hasAccess` remains an external projection, not the lock |
| Crash after business success but before delivery | The business success and outbox insert must be one transaction; dispatcher resumes pending rows | Reconciliation identifies missing or stuck deliveries and provides audited redrive |

The current PoC created none of the PAY-041 tables or behavior. This table is a
contract handoff for a separately reviewed design.

## Boundary and cleanup statement

No production or staging integration occurred. No shared Kubernetes cluster was
accessed. No payment was initiated, no business billing was performed, no
deployment was made, and no data migration ran. No existing usage counter was
changed, shadow-written, switched, or deleted. Running the quickstart's local
`billing-worker` container did not exercise ListingKit billing or payment paths.
No production application behavior or configuration was changed.

The runner removed the local Compose containers and network with `down` without
`-v`; the subsequent project-filtered Docker check returned no containers.
Volumes and the ignored upstream/evidence checkout remain local for
reproducibility. Raw logs, Compose secrets, API keys, and `.local` contents are
excluded from this report commit.
