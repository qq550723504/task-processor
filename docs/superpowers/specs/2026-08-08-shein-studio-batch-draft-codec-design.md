# SHEIN Studio Batch Draft Codec Design

## Status

- Date: 2026-08-08
- Baseline: `master@d4a464eca`
- Scope: behavior-preserving frontend refactor in `web/listingkit-ui`
- Project position: first PR in the staged removal of
  `legacyCompatibilitySnapshot`

## Problem

`src/lib/api/shein-studio-batch-drafts.ts` is approximately 1,200 lines and
currently owns four different concerns:

- HTTP transport for listing, upserting, and deleting batch drafts;
- request serialization from frontend domain models to API payloads;
- response validation and mapping from wire records to domain models;
- compatibility encoding and fallback reads for
  `legacyCompatibilitySnapshot`.

The existing Zod response schema is in a separate file, but it imports a wire
type from the transport module while the transport module imports its parser.
This reverse type dependency makes the apparent separation incomplete.

The compatibility snapshot cannot be deleted safely as part of a mechanical
file split. It is still used to recover designs, approved selections, created
tasks, and generation state from historical drafts. The backend already has
canonical persisted fields for these concepts, but the frontend must first
isolate the old behavior, stop writing it in a later PR, and audit historical
data before removing fallback reads.

## Goals

1. Reduce the existing module to a thin HTTP transport boundary.
2. Separate request encoding from response validation and decoding.
3. Concentrate all compatibility-snapshot transformations behind one clearly
   named temporary adapter.
4. Remove the schema-to-transport reverse type dependency.
5. Migrate callers of pure functions directly to the new codec modules instead
   of preserving the old module surface through re-exports.
6. Preserve request payloads, decoded domain models, error behavior, and every
   existing field-specific fallback rule.
7. Create an explicit seam for later PRs to stop legacy writes and then remove
   legacy reads.

## Non-goals

- Do not remove `legacyCompatibilitySnapshot` in this PR.
- Do not stop writing compatibility data in this PR.
- Do not migrate or backfill stored drafts.
- Do not change the backend API, database schema, or canonical domain models.
- Do not normalize the existing inconsistent empty-value fallback semantics.
- Do not change Studio UI behavior, copy, state ownership, persistence timing,
  generation orchestration, or task creation.
- Do not add a validation or serialization library; reuse Zod and the existing
  API response parser.
- Do not refactor unrelated Studio modules.

## Considered approaches

### 1. Request codec, response codec, legacy adapter, and thin transport — selected

Create one module per direction, isolate legacy behavior in a third module, and
leave the existing file responsible only for HTTP operations.

Benefits:

- establishes narrow, independently testable boundaries;
- gives later removal work a single legacy dependency to target;
- eliminates the current schema/transport dependency cycle;
- allows transport and mapping behavior to evolve independently.

Trade-off: callers of moved pure functions must update imports, and the feature
gains three focused modules. That explicit migration is preferred over keeping
an obsolete facade.

### 2. One bidirectional codec plus thin transport

This creates fewer files, but request encoding, response decoding, and legacy
fallback logic would still share a large module. It would likely reproduce the
same growth and make the legacy boundary difficult to delete independently.

### 3. Zod transforms as the complete bidirectional codec

Zod transforms could decode responses directly to domain objects. Request
encoding and the field-specific legacy precedence rules do not map cleanly to
that model, however. Embedding them in schema transforms would make behavior
less visible and increase regression risk without adding necessary capability.

## Architecture

### Transport

`src/lib/api/shein-studio-batch-drafts.ts` retains only:

- `listSheinStudioBatchDrafts`;
- `upsertSheinStudioBatchDraft`;
- `deleteSheinStudioBatchDraft`;
- transport-facing options that are genuinely part of those operations.

The module selects URL, method, timeout, and client operation. It delegates the
entire upsert body to the request codec and passes untrusted response data to
the response codec. It does not assemble payload fields, normalize records, or
read compatibility snapshots directly.

Moved pure helpers are not re-exported from this module. Transport consumers
continue to import the transport, while mapping consumers move to the relevant
codec.

### Request codec

`src/lib/api/shein-studio-batch-draft-request-codec.ts` owns request-side wire
types, the `UpsertSheinStudioBatchDraftInput` operation input type, and pure
domain-to-wire conversion. Its public entry point is:

```ts
export function buildStudioBatchDraftUpsertPayload(
  input: UpsertSheinStudioBatchDraftInput,
): StudioBatchDraftUpsertPayload;
```

Smaller encoders may be exported only when a real caller or focused test needs
them, such as selection, grouped selection, group, or workspace encoders. The
high-level builder remains the sole payload assembly path used by transport.

The codec preserves current omission semantics, property-presence semantics for
hot-style fields, grouped-selection de-duplication, and all group/workspace wire
shapes. It calls the legacy adapter only to encode the temporary compatibility
payload.

### Response codec

`src/lib/api/shein-studio-batch-draft-response-codec.ts` owns response-side wire
types, Zod schemas, structural parsing, normalizers, and wire-to-domain mapping.
It absorbs the current contents of
`shein-studio-batch-draft-schema.ts`; the old schema file is removed once its
callers migrate.

The public surface contains high-level decoding and mapping operations needed
by transport and existing batch utilities. Representative entry points are:

```ts
export function decodeStudioBatchDraftDetail(
  value: unknown,
): StudioBatchDraftDetailResponse;

export function mapStudioBatchDraftDetailToDraft(
  detail: StudioBatchDraftDetailResponse | null | undefined,
): SheinStudioDraft | null;
```

Detail decoding retains the existing strict Zod validation. List
decoding/mapping retains the current permissive treatment of the top-level
payload; adding strict list validation would be a separate behavior change.
Currently shared selection/group normalizers remain public only where existing
consumers require them. Internal design, prompt, task, and generation-job
normalizers remain private.

The module depends on domain types, Zod, `parseApiResponseShape`, and the legacy
adapter. It never imports the transport module.

### Legacy compatibility adapter

`src/lib/api/shein-studio-batch-draft-legacy-adapter.ts` is the only module that
understands the legacy snapshot wire shape. It owns:

- encoding a domain compatibility snapshot to its request payload;
- decoding a response snapshot to its domain representation;
- applying explicitly selected canonical overrides while producing the domain
  compatibility snapshot returned by current detail mapping;
- determining when an encoded or decoded snapshot is semantically empty.

It does not make HTTP calls and does not choose canonical-versus-legacy
precedence. The response codec owns that field-level choice because the choice
depends on whether it is decoding a detail, list, or nested group record. Where
current behavior stores selected canonical values in the returned domain legacy
snapshot, the response codec passes domain-shaped overrides to the adapter; it
does not construct or merge legacy wire keys itself.

No business component, utility, or transport function may import this adapter
directly. The file and exported names explicitly retain `legacy` so that the
temporary dependency remains searchable and measurable.

## Data flow

### Request

```text
Studio domain input
  -> buildStudioBatchDraftUpsertPayload
  -> focused request encoders
  -> legacy adapter for compatibility snapshot only
  -> thin transport
  -> API
```

Transport never reconstructs or modifies the encoded body. The request codec
returns the complete payload used by the API client.

### Response

```text
Untrusted API JSON
  -> existing detail Zod validation or existing permissive list handling
  -> response normalizers
  -> canonical field mapping
  -> field-specific legacy fallback where required
  -> existing Studio domain model
```

Business code receives the same domain objects it receives today and remains
unaware of response wire names and legacy snapshot layout.

## Preserved compatibility contract

The current implementation does not use one universal fallback rule. This PR
must preserve the following field-specific behavior exactly.

### Detail records

- Designs use the canonical `detail.designs` only when it is non-empty;
  otherwise they fall back to legacy designs.
- Approved selection first uses `approved_design_ids`, or approved design IDs
  derived from canonical designs when that property is nullish. If the resulting
  array is empty, the mapped domain selection falls back to legacy selected IDs.
- Created tasks use nullish fallback from `created_tasks` to the raw legacy
  field. An explicit empty canonical array remains authoritative.
- Generation jobs use a non-empty canonical jobs list first. If it is empty and
  `generation_job_id` exists, decoding synthesizes one running job. Otherwise
  jobs fall back to the legacy snapshot.
- Generation error and generation job ID use nullish fallback. An explicit
  empty string remains authoritative.

### List records

- `approved_design_ids` and `created_tasks` use nullish fallback; explicit empty
  arrays remain authoritative.
- Designs and legacy generation state continue to come from the compatibility
  snapshot because the list response does not contain their complete canonical
  representation.

### Nested group records

- A designs property that is an array is authoritative, including an empty
  array. Legacy designs are used only when the property is not an array.
- Approved or selected ID properties that are arrays are authoritative,
  including empty arrays. Legacy IDs are used only when neither representation
  is an array.
- Existing snake-case/camel-case compatibility remains unchanged.

### Request snapshots

An absent snapshot remains omitted. A snapshot with no designs, selected IDs,
created tasks, generation jobs, generation error, or generation job ID also
encodes to `undefined` and is omitted. Non-empty snapshots retain the current
wire field names and design/job mappings.

These rules are deliberately documented rather than simplified. Semantic
normalization belongs to the later legacy-removal stages after stored-data
coverage is known.

## Error handling

Detail response parsing continues to use `parseApiResponseShape`. A detail
shape failure throws `ApiError` with status `502` and preserves the Zod issue
paths and messages in the error details. List responses retain their current
permissive top-level handling and `items ?? []` behavior; this structural
refactor does not introduce a new list rejection path.

HTTP status failures, timeouts, authentication failures, and other client
errors continue to propagate from the existing API client without codec-level
wrapping. Codecs are synchronous pure transformations and introduce no retries
or side effects.

Legacy snapshot contents retain the current tolerant behavior: invalid nested
items are filtered by focused normalizers when possible rather than making an
otherwise readable historical draft fail as a whole.

## Migration sequence for this PR

1. Add characterization tests at the future codec interfaces before moving
   implementation.
2. Introduce the legacy adapter and move the existing encode/decode behavior
   without semantic changes.
3. Introduce the request codec and route upsert payload construction through its
   high-level builder.
4. Introduce the response codec, move Zod schemas and mapping functions, and
   remove the old schema file.
5. Reduce the original module to transport operations.
6. Update each caller to import pure functions and wire types from their owning
   codec. Do not add compatibility re-exports.
7. Run focused and full regression gates.

Each move is mechanical after its characterization test passes. No cleanup that
changes fallback or omission behavior is combined with the move.

## Testing strategy

Implementation follows red-green-refactor.

### Request codec characterization

Tests compare complete encoded payloads for:

- a representative full upsert;
- omitted optional values and explicit empty arrays;
- hot-style property-presence behavior;
- selection, grouped selection, group, and workspace payloads;
- non-empty and semantically empty compatibility snapshots.

### Response codec characterization

Detail, list, and nested group fixtures cover canonical-only, legacy-only, and
mixed records. A precedence matrix explicitly covers absent properties, `null`,
empty arrays, non-empty arrays, empty strings, and non-empty strings wherever
the current rules distinguish them.

Additional tests preserve:

- grouped primary-selection de-duplication;
- hot-style reference property presence;
- created-task normalization;
- generation-job synthesis from `generation_job_id`;
- filtering of invalid legacy nested items;
- `502 ApiError` shape and issue paths for invalid detail responses;
- permissive list handling when the `items` property is missing, as tolerated by
  the current implementation.

### Transport tests

Transport tests assert URL, HTTP method, timeout, client operation, and that the
body returned by the request codec is passed through unchanged. They do not
duplicate codec mapping assertions.

### Dependency and regression gates

Review and static checks must show that:

- the response codec does not import transport;
- transport and business modules do not import the legacy adapter;
- the original transport does not re-export moved pure functions;
- no old schema imports remain.

Run focused API and Studio utility suites while developing, followed by:

```powershell
npm.cmd run lint
npm.cmd run typecheck
npm.cmd test
npm.cmd run build
```

from `web/listingkit-ui`. Run the repository-level backend regression gate with
workspace mode disabled:

```powershell
$env:GOWORK='off'
go test ./... -count=1
```

## Acceptance criteria

- `shein-studio-batch-drafts.ts` contains only transport responsibilities.
- Request encoding, response decoding, and legacy snapshot conversion have one
  named owner each.
- The response schema no longer imports types from transport, and the old schema
  module is removed.
- Callers import moved pure functions directly from codec modules; no facade
  re-exports preserve the old boundary.
- Characterization tests cover every documented empty-value and legacy
  precedence rule.
- Encoded request fixtures and decoded domain fixtures are equivalent to the
  pre-refactor behavior.
- No API contract, stored-data mutation, UI behavior, or orchestration behavior
  changes.
- Frontend lint, typecheck, tests, and build pass; repository Go tests remain
  green.

## Follow-up project stages

The legacy snapshot removal continues through separate designs and PRs:

1. Stop frontend writes after proving canonical fields cover every current
   writer and recovery path.
2. Audit historical records and backfill or explicitly retire records that rely
   only on compatibility data.
3. Remove legacy fallback reads, adapter code, snapshot types, fixtures, and
   dead compatibility branches.

This design authorizes only the codec-boundary PR. It does not imply that later
stages may proceed without their own data evidence and review.
