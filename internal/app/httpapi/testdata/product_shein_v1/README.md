# Product to SHEIN shared regression v1

Refs #335 / #47 / #44. This is the first bounded repository regression slice,
not production source collection, remote publishing, human approval or Agent eval.

`manifest.json` identifies dataset `product-shein-shared`, version `1.0.0`, the
SHA-256 of the **exact UTF-8 bytes of cases.json including its final LF**, fixed
semantic time, rule and binding versions. Local attributes pin JSON to LF on
Windows and Linux. The digest excludes the manifest itself and this README.
The loader verifies bytes, unique stable IDs, count and pinned version before
either consumer runs. There is no golden-update or accept-all command.

All source/evidence/URLs and package contents are authored synthetic fixtures.
The neutral envelope uses the current 1688 source platform identity without
contacting 1688. Reserved `.example.test` hosts are never fetched. The source
checksum is an opaque synthetic upstream evidence label, not an assertion that
a raw provider response was collected. No customer data, credentials or cookies.
The envelope schema uses the exported Go SourceEnvelope field names; nested
Catalog/Package objects use their current wire JSON contracts.

| case_id | Scenario | Consumers |
| --- | --- | --- |
| PS-001 | Source facts, category normalization, raw lineage, candidate image | deterministic + PostgreSQL |
| PS-002 | Missing title/images with explicit warnings; no invented facts | deterministic + PostgreSQL |
| PS-003 | Missing brand, duplicate warnings, deduplicated review reasons | deterministic + PostgreSQL |
| PS-004 | Missing source platform: strict identity rejection | deterministic only; no valid Publisher input |
| PS-005 | Explicit synthetic complete offline Package | evaluator only; current real assembler cannot supply its template/approved-asset prerequisites |
| PS-006 | Synthetic warning-only manual notes | evaluator only, same missing real prerequisites |
| PS-007 | Synthetic unconfirmed final draft: publish vs save_draft | evaluator only, same missing real prerequisites |
| PS-008 | Fixed empty binding vector, equivalent null/empty semantics, changed content rejection | evaluator only; exact synthetic persisted bytes |
| PS-009 | Trusted caller seam: fixed expiry boundary rejects with retained evidence | evaluator only; real producer has no freshness evidence |

## Oracle and changes

Expected snapshots and rule assertions were authored from approved #319
D1-CURRENT-PRODUCT-INPUT/V1, #315's diagnostic contract, and #318's approved
input-binding design. Existing independent evidence:

- `internal/product/sourcing/normalize_test.go` and `catalog_asset_handoff_test.go`:
  raw evidence, lineage, warning normalization, candidate-image traces, missing facts.
- `internal/listing/record/README.md`: Publisher/POST ownership, receipt, immutable
  source version, replay/conflict, scope, no image approval and incomplete drafts.
- `internal/marketplace/shein/validator/validator_test.go`: ready/warning-only and
  final-confirmation action differences; complete Package is explicitly synthetic.
- `internal/marketplace/shein/validator/diagnostic_test.go`:
  `TestDiagnosticFixedEncodingVectors` supplies the already-reviewed empty vector.
- `docs/superpowers/specs/2026-09-05-shein-offline-validation-input-binding-design.md`
  section 3 and current `internal/marketplace/validator/diagnostic.go`: content
  binding, expiry equality, unknown coverage, zero report on error.

Independent read-only reviewer `oracle_review` checked these assertions before
the first semantic execution. Final exact-SHA review evidence is maintained on
the PR. No evaluator output was used to populate expected answers. Repeated
evaluation/receipt/report comparisons are **additional consistency assertions**,
not substitutes for the shared manual oracle.

Synthetic complete packages use exact blocker/warning sets. Real incomplete
packages assert five mandatory template/image blocker rule+code pairs; other
current assembler findings remain visible and are not required to be absent.
This deliberately does not claim a complete golden for unstable presentation
text or every downstream rule. All returned blocker/warning entries must also
exist in checks and retain category/paths/guidance. Both actions must retain
unknown freshness and the exact eight unassessed scopes. Draft's policy never
removes mandatory blockers. A ready offline result is never submission authority.
Complete synthetic cases also require the exact 15-rule checklist, so dropping
a successful check cannot produce a false pass. The loader rejects empty outcome
branches, missing inputs, duplicate/unknown actions/layers and incomplete PG cases.

When changing data/expected, explain the changed contract and case IDs, obtain
independent review, bump the dataset version, update the exact cases hash and
loader's pinned version/count if necessary. Rule/binding changes need their
owner's version decision. Do not modify Catalog/Validator rules to fit fixtures.

## Execution and environment evidence

From the repository root, normal Go/CI discovery runs the deterministic consumer:

```powershell
go test -v ./internal/app/httpapi -run '^TestSharedRegression' -count=1
```

Each run logs dataset/version/hash, actual Git HEAD, rule/binding, layer, then Go
PASS/FAIL/SKIP and case results. During development HEAD alone does not identify
uncommitted changes; only clean final-HEAD runs count as candidate evidence.
Absent `ISSUE376_TEST_DSN` yields explicit PostgreSQL **SKIP**, never acceptance
PASS. Existing backend CI runs `go test ./...`; no PostgreSQL service/DSN is
configured there. Its default SKIP is recorded separately from controlled PG
evidence. No CI policy, classifier, Required Gate or runner is changed.

Use only a new task-owned container. Example PowerShell commands (ephemeral
password is synthetic, not a shared credential):

```powershell
$qaName = 'issue335-pg-' + [guid]::NewGuid().ToString('N')
docker run -d --name $qaName -e POSTGRES_PASSWORD=issue335-isolated -e POSTGRES_DB=issue335 -p 127.0.0.1::5432 postgres:16-alpine
try {
  $qaPort = (docker port $qaName 5432).Split(':')[-1]
  for ($qaAttempt=0; $qaAttempt -lt 30; $qaAttempt++) {
    docker exec $qaName pg_isready -U postgres -d issue335
    if ($LASTEXITCODE -eq 0) { break }
    Start-Sleep -Seconds 1
  }
  if ($LASTEXITCODE -ne 0) { throw 'Isolated PostgreSQL failed readiness' }
  $env:ISSUE376_TEST_DSN = "host=127.0.0.1 port=$qaPort user=postgres password=issue335-isolated dbname=issue335 sslmode=disable"
  go test -race -v ./internal/app/httpapi -run '^TestSharedRegression' -count=1
  if ($LASTEXITCODE -ne 0) { throw 'Shared regression failed' }
  go test -race ./internal/app/httpapi -run '^TestShein(Diagnostic|Record)' -count=1
  if ($LASTEXITCODE -ne 0) { throw 'Existing adjacent regression failed' }
} finally {
  Remove-Item Env:ISSUE376_TEST_DSN -ErrorAction SilentlyContinue
  docker stop $qaName
  docker rm $qaName
}
```

Stop the foreground Go process with Ctrl+C if needed, then stop/remove only the
container whose exact name was saved in `$qaName`. The reused `recordTestDB`
creates/drops random `issue319_<UUID>` schemas (historical helper name only).
Normal `t.Cleanup` removes schemas and closes pools; interrupted runs may leave
schemas until the task-owned container is removed. Never point this variable at
shared/production databases. The fixture never manually inserts successful
Listing records and uses no fake repository/static HTTP response.

PG exercises real SourceEnvelope conversion -> Catalog Publisher -> TCP HTTP
POST -> GET -> close server/reconstruct repositories and application -> GET and
POST replay. Only `recordVerifier` and grant acquisition are external identity
substitutes; actual Organization middleware and configured authorizer execute.
The extra switcher identity has real policy-evaluated grants in two organizations.
Separate setups create same-operation records in both scopes, then switch
200 -> 300 -> 200 and prove wrong-scope nonexistence.

UUID syntax, receipt/resource/operation/org/owner/product/version linkage,
persisted bytes and created_at are checked. Read/evaluation times must lie within
actual HTTP request boundaries and remain ordered; new reads may advance only
those clocks. No external observed_at/valid_until is inferred from runtime time.
Catalog content/receipts, business rows and PostgreSQL xmin are checked before
and after GET/replay/denied operations; legitimate setup writes occur outside
that window. No dynamic IDs, version, digest or owner is dropped to obtain PASS.

Future Apply, actual asset approval, remote save/publish/unknown-state recovery,
Next/BFF/browser, real identity/provider acceptance and paid AI are NOT_RUN/out
of this slice. Final HEAD, CI runs, PG logs and independent review live on the PR;
#47/#44/#33 stay open and merge/deployment remain separately authorized.

## Preexisting assembler ordering defect handed to its owner

Classification: BACKLOG for Marketplace/publishing assembler owner, not a change
to the approved binding contract. Two attributes in the same fixed Catalog
snapshot can be persisted in different array orders on separate builds:

```text
input: Title=Bottle, Attributes=[{alpha:first}, {zeta:last}]
build 0:  product_attributes=[{"name":"alpha","value":"first"},{"name":"zeta","value":"last"}]
build 20: product_attributes=[{"name":"zeta","value":"last"},{"name":"alpha","value":"first"}]
```

Minimal failing evidence was obtained on the main production baseline with a
temporary Go test calling `draft.Builder.Build` up to 100 times with that exact
`catalog.ProductSnapshot` and `record.Input{Country:"US", Language:"en"}`,
decoding each persisted Package and comparing `json.Marshal(pkg.ProductAttributes)`
to the first build. It failed at build 20. No evaluator was used as an oracle.
The temporary reproducer is not included in the passing default suite.

Root cause: `catalog.ProjectCanonical` converts attributes to a map;
`publishing/common.BuildAttributes` iterates it without sorting back to a slice;
the assembler persists this slice. Binding intentionally preserves arrays, so
different persisted bytes correctly have different digests. Do not sort evaluator
input or change expected to hide this. Same persisted bytes/action/binding remain
deterministic and stable across restart. This suite does not assert byte equality
between independent new assembler builds, or let this defect excuse a changed
digest on a re-read of the same record. No listed Blocker consequence was found;
fixing the assembler's ordering is outside #335's test-only authority.
