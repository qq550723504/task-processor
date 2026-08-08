# SHEIN Studio Batch Draft Codecs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split SHEIN Studio batch-draft transport, request encoding, response decoding, and legacy compatibility behavior into acyclic, directly testable modules without changing API or historical-draft behavior.

**Architecture:** Keep `shein-studio-batch-drafts.ts` as the thin HTTP boundary. Route all domain-to-wire conversion through a request codec, all wire-to-domain conversion through a response codec, and all legacy snapshot conversion through a temporary adapter; share only genuinely cross-path pure helpers through an acyclic codec-primitives module.

**Tech Stack:** TypeScript 6, Vitest 4, Zod 4, Next.js 16, existing `apiRequest`/`parseApiResponseShape`, Go repository regression tests.

## Global Constraints

- Preserve every request payload field, omission rule, response default, and field-specific canonical-versus-legacy precedence rule documented in the approved spec.
- Do not remove or stop writing `legacyCompatibilitySnapshot` in this PR.
- Do not change backend APIs, database schemas, domain models, UI behavior, copy, state ownership, persistence timing, generation orchestration, or task creation.
- Reuse Zod and existing helpers; add no dependency.
- Keep list responses permissive as today; only detail responses use strict Zod parsing and `502 ApiError` failures.
- Do not re-export moved pure functions from `shein-studio-batch-drafts.ts`.
- At the final Task 5 boundary, business components, utilities, and transport must not import the legacy adapter or codec-primitives module directly; temporary imports in the original mixed module are removed when mapping moves.
- Use TDD for every extraction: add the future-interface test, observe the expected failure, then move the existing behavior.
- Stage only the files listed by each task and review `git diff --cached` before every commit.

---

## File Structure

### Files to create

- `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-codec-primitives.ts`
  - Pure helpers shared across request, response, and legacy decoding.
- `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-codec-primitives.test.ts`
  - Characterization coverage for name, URL, design, task, and job normalization.
- `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-legacy-adapter.ts`
  - Legacy snapshot wire encoding/decoding and domain-shaped canonical overrides.
- `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-legacy-adapter.test.ts`
  - Empty-snapshot, round-trip, invalid-item, and override-presence coverage.
- `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-request-codec.ts`
  - Upsert input contract, complete payload builder, selection/group/workspace encoders, and selection identity key.
- `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-request-codec.test.ts`
  - Full fixture equality and request property-presence characterization.
- `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-response-codec.ts`
  - Response wire types, detail Zod schema/parser, permissive list mapping, and all public wire-to-domain mapping.
- `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-response-codec.test.ts`
  - Detail/list mapping, fallback matrix, parser error, hot-style presence, and grouped selection coverage.

### Files to modify

- `web/listingkit-ui/src/lib/api/shein-studio-batch-drafts.ts`
  - Remove conversion logic and retain list/upsert/delete transport only.
- `web/listingkit-ui/src/lib/api/shein-studio-batches.ts`
  - Import selection/group response normalizers from the response codec.
- `web/listingkit-ui/src/lib/api/shein-studio.test.ts`
  - Import transport functions and response codec functions from their new owners.
- `web/listingkit-ui/src/lib/api/__fixtures__/shein-studio-batch-contract.ts`
  - Import `UpsertSheinStudioBatchDraftInput` from the request codec.
- `web/listingkit-ui/src/lib/utils/shein-studio-batches.ts`
  - Import the selection key from the request codec and HTTP functions from transport.
- `web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts`
  - Mock request-codec selection-key behavior separately from transport behavior.

### File to delete

- `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-schema.ts`
  - Its Zod contract moves into the response codec, eliminating the reverse type dependency.

---

### Task 1: Extract acyclic codec primitives

**Files:**

- Create: `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-codec-primitives.ts`
- Create: `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-codec-primitives.test.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batch-drafts.ts:250-270, 577-650, 968-1042`

**Interfaces:**

- Consumes: domain types from `@/lib/types/shein-studio` only.
- Produces:

```ts
export function deriveStudioBatchDraftName(prompt: string): string;
export function normalizeStudioHotStyleReferenceImageUrls(value: unknown): string[];
export function normalizeStudioBatchCreatedTasks(
  input: unknown,
  fallbackDesignIds?: string[],
  fallbackDesigns?: Array<{ id: string } | undefined>,
): SheinStudioCreatedTask[];
export function normalizeStudioBatchGenerationJobs(
  input: unknown,
): SheinStudioGenerationJob[];
export function normalizeStudioBatchDesignResponse(
  design: Record<string, unknown>,
): SheinStudioGeneratedDesign | null;
```

- [ ] **Step 1: Add failing characterization tests for shared primitives**

Create the test file with explicit snake-case/camel-case and invalid-item cases:

```ts
import { describe, expect, it } from "vitest";

import {
  deriveStudioBatchDraftName,
  normalizeStudioBatchCreatedTasks,
  normalizeStudioBatchDesignResponse,
  normalizeStudioBatchGenerationJobs,
  normalizeStudioHotStyleReferenceImageUrls,
} from "@/lib/api/shein-studio-batch-draft-codec-primitives";

describe("SHEIN Studio batch draft codec primitives", () => {
  it("preserves name and hot-style normalization", () => {
    expect(deriveStudioBatchDraftName("  retro cherries  ")).toBe("retro cherries");
    expect(deriveStudioBatchDraftName("x".repeat(40))).toBe(`${"x".repeat(36)}...`);
    expect(
      normalizeStudioHotStyleReferenceImageUrls([
        "  https://example.com/a.png  ",
        "https://example.com/a.png",
        "https://example.com/b.png",
      ]),
    ).toEqual(["https://example.com/a.png"]);
  });

  it("normalizes reusable design, task, and job records", () => {
    expect(
      normalizeStudioBatchDesignResponse({
        id: "design-1",
        image_url: "https://example.com/design.png",
        revisedPrompt: "revised",
        variation_intensity: "medium",
      }),
    ).toMatchObject({
      id: "design-1",
      imageUrl: "https://example.com/design.png",
      revisedPrompt: "revised",
      variationIntensity: "medium",
    });
    expect(
      normalizeStudioBatchCreatedTasks(
        [{ id: "task-1", title: "Create", design_id: "design-1" }, null],
        [],
        [],
      ),
    ).toEqual([{ id: "task-1", title: "Create", designId: "design-1" }]);
    expect(
      normalizeStudioBatchGenerationJobs([
        { job_id: " job-1 ", status: "succeeded" },
        { job_id: "", status: "failed" },
      ]),
    ).toEqual([{ jobId: "job-1", status: "succeeded" }]);
  });
});
```

- [ ] **Step 2: Run the new test and observe the missing-module failure**

Run from `web/listingkit-ui`:

```powershell
npm.cmd test -- src/lib/api/shein-studio-batch-draft-codec-primitives.test.ts
```

Expected: FAIL because `shein-studio-batch-draft-codec-primitives` does not exist.

- [ ] **Step 3: Move the five existing helper implementations into the primitives module**

Move the existing bodies without changing trimming, length, filtering, fallback,
or status-default rules. Rename only at the exported boundary shown above. Keep
the task helper's internal non-blank-string helper private:

```ts
function asNonBlankString(value: unknown) {
  return typeof value === "string" && value.trim() ? value : undefined;
}
```

In `shein-studio-batch-drafts.ts`, import these functions and replace every old
call before deleting the old private definitions. This keeps the existing
public API working while proving the extraction independently.

- [ ] **Step 4: Run focused primitive and existing API tests**

```powershell
npm.cmd test -- `
  src/lib/api/shein-studio-batch-draft-codec-primitives.test.ts `
  src/lib/api/shein-studio.test.ts
```

Expected: both files PASS.

- [ ] **Step 5: Commit the primitive extraction**

```powershell
git add -- `
  web/listingkit-ui/src/lib/api/shein-studio-batch-draft-codec-primitives.ts `
  web/listingkit-ui/src/lib/api/shein-studio-batch-draft-codec-primitives.test.ts `
  web/listingkit-ui/src/lib/api/shein-studio-batch-drafts.ts
git diff --cached --check
git diff --cached
git commit -m "refactor: extract batch draft codec primitives"
```

---

### Task 2: Isolate the legacy compatibility adapter

**Files:**

- Create: `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-legacy-adapter.ts`
- Create: `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-legacy-adapter.test.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batch-drafts.ts:304-430, 519-570, 731-865, 1044-1205`

**Interfaces:**

- Consumes: the Task 1 primitive normalizers and `SheinStudioLegacyCompatibilitySnapshot`.
- Produces:

```ts
export type StudioBatchDraftLegacySnapshotOverrides = Partial<
  SheinStudioLegacyCompatibilitySnapshot
>;

export function encodeStudioBatchDraftLegacySnapshot(
  snapshot: SheinStudioLegacyCompatibilitySnapshot | undefined,
): Record<string, unknown> | undefined;

export function decodeStudioBatchDraftLegacySnapshot(
  value: Record<string, unknown> | undefined,
): SheinStudioLegacyCompatibilitySnapshot | undefined;

export function mergeStudioBatchDraftLegacySnapshot(
  snapshot: SheinStudioLegacyCompatibilitySnapshot | undefined,
  overrides: StudioBatchDraftLegacySnapshotOverrides,
): SheinStudioLegacyCompatibilitySnapshot | undefined;
```

`overrides` is domain-shaped. The adapter applies a key only when that key is
an own property of `overrides`, so `createdTasks: []` and `generationError: ""`
remain authoritative while an omitted key leaves the decoded legacy value.

- [ ] **Step 1: Add failing encode/decode and override-presence tests**

```ts
import { describe, expect, it } from "vitest";

import {
  decodeStudioBatchDraftLegacySnapshot,
  encodeStudioBatchDraftLegacySnapshot,
  mergeStudioBatchDraftLegacySnapshot,
} from "@/lib/api/shein-studio-batch-draft-legacy-adapter";

describe("SHEIN Studio batch draft legacy adapter", () => {
  it("omits absent and semantically empty snapshots", () => {
    expect(encodeStudioBatchDraftLegacySnapshot(undefined)).toBeUndefined();
    expect(
      encodeStudioBatchDraftLegacySnapshot({
        designs: [],
        selectedIds: [],
        createdTasks: [],
        generationJobs: [],
      }),
    ).toBeUndefined();
  });

  it("encodes existing legacy wire names", () => {
    expect(
      encodeStudioBatchDraftLegacySnapshot({
        designs: [{ id: "design-1", imageUrl: "https://example.com/1.png" }],
        selectedIds: ["design-1"],
        createdTasks: [],
        generationJobs: [{ jobId: "job-1", status: "running" }],
        generationError: "failed",
        generationJobId: "job-1",
      }),
    ).toMatchObject({
      approved_design_ids: ["design-1"],
      generation_error: "failed",
      generation_job_id: "job-1",
      generation_jobs: [{ job_id: "job-1", status: "running" }],
      designs: [{ id: "design-1", image_url: "https://example.com/1.png" }],
    });
  });

  it("applies explicit empty canonical overrides without erasing omitted values", () => {
    const legacy = decodeStudioBatchDraftLegacySnapshot({
      approved_design_ids: ["legacy-design"],
      created_tasks: [{ id: "legacy-task", title: "Legacy" }],
      generation_error: "legacy-error",
    });
    expect(
      mergeStudioBatchDraftLegacySnapshot(
        legacy,
        { createdTasks: [], generationError: "" },
      ),
    ).toMatchObject({
      selectedIds: ["legacy-design"],
      createdTasks: [],
      generationError: "",
    });
  });
});
```

- [ ] **Step 2: Run the adapter test and observe the missing-module failure**

```powershell
npm.cmd test -- src/lib/api/shein-studio-batch-draft-legacy-adapter.test.ts
```

Expected: FAIL because the adapter module does not exist.

- [ ] **Step 3: Move legacy wire knowledge into the adapter**

Move `legacyCompatibilitySnapshotToPayload` and
`normalizeLegacyCompatibilitySnapshotResponse` into the new module. Replace
nested normalization with Task 1 imports. Apply overrides with a presence-aware
helper:

```ts
const LEGACY_SNAPSHOT_KEYS = [
  "designs",
  "selectedIds",
  "createdTasks",
  "generationJobs",
  "generationError",
  "generationJobId",
] as const;

const merged = { ...(snapshot ?? {}) };
for (const key of LEGACY_SNAPSHOT_KEYS) {
  if (Object.prototype.hasOwnProperty.call(overrides, key)) {
    Object.assign(merged, { [key]: overrides[key] });
  }
}
```

`decodeStudioBatchDraftLegacySnapshot` normalizes only the raw legacy value.
`mergeStudioBatchDraftLegacySnapshot` clones that domain value, applies each
own-property override, and then performs the existing semantic-empty check. Do
not emit an empty decoded or merged snapshot.

- [ ] **Step 4: Replace old-module calls with the adapter and explicit overrides**

For detail mapping, decode the raw snapshot first. Move the existing canonical
design mapping block above legacy composition, then calculate effective
selection/design arrays before normalizing canonical created tasks. Finally
merge only the overrides selected by the documented field-specific precedence:

```ts
const decodedLegacySnapshot = decodeStudioBatchDraftLegacySnapshot(
  rawBatchLegacyCompatibilitySnapshot,
);
const snapshotSelectedIds =
  selectedIds.length > 0
    ? selectedIds
    : (decodedLegacySnapshot?.selectedIds ?? []);
const snapshotDesigns =
  normalizedCanonicalDesigns.length > 0
    ? normalizedCanonicalDesigns
    : (decodedLegacySnapshot?.designs ?? []);

const legacyCompatibilitySnapshot = mergeStudioBatchDraftLegacySnapshot(
  decodedLegacySnapshot,
  {
    ...(selectedIds.length > 0 ? { selectedIds } : {}),
    ...(detail.batch.created_tasks !== undefined
      ? {
          createdTasks: normalizeStudioBatchCreatedTasks(
            detail.batch.created_tasks,
            snapshotSelectedIds,
            snapshotDesigns,
          ),
        }
      : {}),
    ...(normalizedCanonicalJobs.length > 0
      ? { generationJobs: normalizedCanonicalJobs }
      : detail.batch.generation_job_id
        ? { generationJobs: [{ jobId: detail.batch.generation_job_id, status: "running" }] }
        : {}),
    ...(detail.batch.generation_error !== undefined
      ? { generationError: detail.batch.generation_error }
      : {}),
    ...(detail.batch.generation_job_id !== undefined
      ? { generationJobId: detail.batch.generation_job_id }
      : {}),
    ...(normalizedCanonicalDesigns.length > 0
      ? { designs: normalizedCanonicalDesigns }
      : {}),
  },
);
```

Use decode alone for list and nested-group legacy snapshots, then retain their
current call-site precedence. Delete both old private legacy functions only
after all call sites use the adapter.

- [ ] **Step 5: Run adapter and legacy regression coverage**

```powershell
npm.cmd test -- `
  src/lib/api/shein-studio-batch-draft-legacy-adapter.test.ts `
  src/lib/api/shein-studio.test.ts
```

Expected: PASS, including grouped and top-level legacy restoration tests.

- [ ] **Step 6: Commit the legacy boundary**

```powershell
git add -- `
  web/listingkit-ui/src/lib/api/shein-studio-batch-draft-legacy-adapter.ts `
  web/listingkit-ui/src/lib/api/shein-studio-batch-draft-legacy-adapter.test.ts `
  web/listingkit-ui/src/lib/api/shein-studio-batch-drafts.ts
git diff --cached --check
git diff --cached
git commit -m "refactor: isolate batch draft legacy adapter"
```

---

### Task 3: Extract the request codec and route upserts through it

**Files:**

- Create: `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-request-codec.ts`
- Create: `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-request-codec.test.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batch-drafts.ts:163-247, 652-763`
- Modify: `web/listingkit-ui/src/lib/api/__fixtures__/shein-studio-batch-contract.ts:1`
- Modify: `web/listingkit-ui/src/lib/utils/shein-studio-batches.ts:1-7`
- Modify: `web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts:13-35`

**Interfaces:**

- Consumes: Task 1 name/URL primitives, Task 2 legacy encoder, SDS payload types.
- Produces:

```ts
export type UpsertSheinStudioBatchDraftInput = Omit<
  SheinStudioPersistedBatchView,
  "updatedAt" | "sheinStoreId"
> & {
  id?: string;
  expectedUpdatedAt?: string;
  name?: string;
  sheinStoreId?: string;
};

export function buildStudioBatchDraftSelectionKey(
  selection?: SDSProductVariantSelection,
): string;

export function buildStudioBatchDraftUpsertPayload(
  input: UpsertSheinStudioBatchDraftInput,
): Record<string, unknown>;
```

- [ ] **Step 1: Add failing request fixture and omission tests**

```ts
import { describe, expect, it } from "vitest";

import { sheinStudioBatchUpsertContractFixture } from "@/lib/api/__fixtures__/shein-studio-batch-contract";
import { buildStudioBatchDraftUpsertPayload } from "@/lib/api/shein-studio-batch-draft-request-codec";

describe("SHEIN Studio batch draft request codec", () => {
  it("encodes the established upsert contract", () => {
    const wirePayload = JSON.parse(
      JSON.stringify(
        buildStudioBatchDraftUpsertPayload(sheinStudioBatchUpsertContractFixture.input),
      ),
    );
    expect(
      wirePayload,
    ).toEqual(sheinStudioBatchUpsertContractFixture.expectedBody);
  });

  it("does not synthesize a name for an existing unnamed batch", () => {
    const payload = buildStudioBatchDraftUpsertPayload({
      ...sheinStudioBatchUpsertContractFixture.input,
      id: "batch-1",
      name: " ",
      legacyCompatibilitySnapshot: undefined,
    });
    expect(payload.batch_name).toBeUndefined();
    expect(payload.legacy_compatibility_snapshot).toBeUndefined();
    const wirePayload = JSON.parse(JSON.stringify(payload));
    expect(wirePayload).not.toHaveProperty("batch_name");
    expect(wirePayload).not.toHaveProperty("legacy_compatibility_snapshot");
  });

  it("preserves explicit hot-style field presence", () => {
    const payload = buildStudioBatchDraftUpsertPayload({
      ...sheinStudioBatchUpsertContractFixture.input,
      hotStyleReferenceImageUrls: [" https://example.com/hot.png "],
      hotStyleReferenceBrief: "brief",
      hotStyleReferencePrompt: "prompt",
    });
    expect(payload).toMatchObject({
      hot_style_reference_image_urls: ["https://example.com/hot.png"],
      hot_style_reference_brief: "brief",
      hot_style_reference_prompt: "prompt",
    });
  });
});
```

- [ ] **Step 2: Run the request test and observe the missing-module failure**

```powershell
npm.cmd test -- src/lib/api/shein-studio-batch-draft-request-codec.test.ts
```

Expected: FAIL because the request codec does not exist.

- [ ] **Step 3: Move request types and encoders into the request codec**

Move the input type, selection key, selection payload encoder, grouped-selection
encoder, grouped-workspace encoder, and complete POST body assembly. Preserve
the current batch-name rule exactly:

```ts
const explicitBatchName = input.name?.trim() || undefined;
const batchName =
  explicitBatchName ??
  (input.id ? undefined : deriveStudioBatchDraftName(input.prompt));
```

The payload builder must return the exact body that transport currently passes
to `apiRequest`, including properties whose value is `undefined`. Do not clean
or compact the object.

- [ ] **Step 4: Make upsert transport pass the complete codec body unchanged**

Replace inline body assembly with:

```ts
body: buildStudioBatchDraftUpsertPayload(input),
```

Import `UpsertSheinStudioBatchDraftInput` into transport for the function
signature; do not re-export it from transport.

- [ ] **Step 5: Migrate the fixture and selection-key callers**

Update the contract fixture type import to the request codec. In
`lib/utils/shein-studio-batches.ts`, split imports so only list/upsert/delete
come from transport and `buildStudioBatchDraftSelectionKey` comes from the
request codec.

In `lib/utils/shein-studio-batches.test.ts`, keep the three transport mocks in
the existing transport mock and add a separate request-codec mock:

```ts
vi.mock("@/lib/api/shein-studio-batch-draft-request-codec", () => ({
  buildStudioBatchDraftSelectionKey: (...args: unknown[]) =>
    buildStudioBatchDraftSelectionKey(...args),
}));
```

- [ ] **Step 6: Run request, transport, and utility regression tests**

```powershell
npm.cmd test -- `
  src/lib/api/shein-studio-batch-draft-request-codec.test.ts `
  src/lib/api/shein-studio.test.ts `
  src/lib/utils/shein-studio-batches.test.ts
```

Expected: all three files PASS.

- [ ] **Step 7: Commit the request codec**

```powershell
git add -- `
  web/listingkit-ui/src/lib/api/shein-studio-batch-draft-request-codec.ts `
  web/listingkit-ui/src/lib/api/shein-studio-batch-draft-request-codec.test.ts `
  web/listingkit-ui/src/lib/api/shein-studio-batch-drafts.ts `
  web/listingkit-ui/src/lib/api/__fixtures__/shein-studio-batch-contract.ts `
  web/listingkit-ui/src/lib/utils/shein-studio-batches.ts `
  web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts
git diff --cached --check
git diff --cached
git commit -m "refactor: extract batch draft request codec"
```

---

### Task 4: Move the response contract and Zod parser

**Files:**

- Create: `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-response-codec.ts`
- Create: `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-response-codec.test.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batch-drafts.ts:32-153`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio.test.ts:1-14`
- Delete: `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-schema.ts`

**Interfaces:**

- Consumes: Zod, `parseApiResponseShape`, and current Studio domain wire-field types.
- Produces in this task:

```ts
export type StudioBatchDraftStatus =
  | "selecting"
  | "generating"
  | "generated"
  | "reviewing"
  | "failed"
  | "tasks_created";

export type StudioBatchDraftDetailResponse = {
  batch?: StudioBatchDraftRecordResponse;
  designs?: StudioBatchDraftDesignResponse[];
};

export type StudioBatchListResponse = {
  items?: StudioBatchListItemResponse[];
};

export function parseStudioBatchDraftDetailResponse(
  payload: unknown,
): StudioBatchDraftDetailResponse;
```

Keep record/list/design/raw-task types exported only when the transport or a
later response-mapping function needs them.

- [ ] **Step 1: Add failing detail parser tests at the new owner**

```ts
import { describe, expect, it } from "vitest";

import { sheinStudioBatchDraftDetailContractFixture } from "@/lib/api/__fixtures__/shein-studio-batch-contract";
import { parseStudioBatchDraftDetailResponse } from "@/lib/api/shein-studio-batch-draft-response-codec";

describe("SHEIN Studio batch draft response codec", () => {
  it("parses the established detail contract", () => {
    expect(
      parseStudioBatchDraftDetailResponse(
        sheinStudioBatchDraftDetailContractFixture.response,
      ),
    ).toMatchObject(sheinStudioBatchDraftDetailContractFixture.response);
  });

  it("reports invalid detail fields as a 502 API shape error", () => {
    expect(() =>
      parseStudioBatchDraftDetailResponse({ batch: { id: 123 } }),
    ).toThrowError(
      expect.objectContaining({
        status: 502,
        payload: expect.objectContaining({
          issues: expect.arrayContaining([
            expect.objectContaining({ path: "batch.id" }),
          ]),
        }),
      }),
    );
  });
});
```

- [ ] **Step 2: Run the response test and observe the missing-module failure**

```powershell
npm.cmd test -- src/lib/api/shein-studio-batch-draft-response-codec.test.ts
```

Expected: FAIL because the response codec does not exist.

- [ ] **Step 3: Move wire types and the current schema into the response codec**

Move response types from `shein-studio-batch-drafts.ts` and move the Zod schemas
plus parser from `shein-studio-batch-draft-schema.ts` without tightening any
field. Preserve `.passthrough()` and the exact error message:

```ts
return parseApiResponseShape(
  payload,
  studioBatchDraftDetailSchema,
  "ListingKit API returned an unexpected studio batch draft response",
) as StudioBatchDraftDetailResponse;
```

- [ ] **Step 4: Redirect imports and delete the old schema file**

Have transport import response types and the parser from the response codec.
Update `shein-studio.test.ts` to import the parser from the response codec. Then
delete `shein-studio-batch-draft-schema.ts`; do not leave a forwarding module.

- [ ] **Step 5: Run parser, API, and type checks**

```powershell
npm.cmd test -- `
  src/lib/api/shein-studio-batch-draft-response-codec.test.ts `
  src/lib/api/shein-studio.test.ts
npm.cmd run typecheck
```

Expected: tests and typecheck PASS.

- [ ] **Step 6: Commit the response contract move**

```powershell
git add -- `
  web/listingkit-ui/src/lib/api/shein-studio-batch-draft-response-codec.ts `
  web/listingkit-ui/src/lib/api/shein-studio-batch-draft-response-codec.test.ts `
  web/listingkit-ui/src/lib/api/shein-studio-batch-drafts.ts `
  web/listingkit-ui/src/lib/api/shein-studio.test.ts `
  web/listingkit-ui/src/lib/api/shein-studio-batch-draft-schema.ts
git diff --cached --check
git diff --cached
git commit -m "refactor: move batch draft response contract"
```

---

### Task 5: Move response mapping and finish the thin transport

**Files:**

- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-response-codec.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batch-draft-response-codec.test.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batch-drafts.ts:1-1205`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batches.ts:1-7`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio.test.ts:1-14`

**Interfaces:**

- Consumes: Task 1 primitives, Task 2 legacy adapter, Task 4 response types/parser, current domain normalization utilities.
- Produces:

```ts
export function mapStudioBatchDraftDetailToDraft(
  detail: StudioBatchDraftDetailResponse | null | undefined,
): SheinStudioDraft | null;

export function mapStudioBatchDraftDetailToBatch(
  detail: StudioBatchDraftDetailResponse | null | undefined,
): SheinStudioSavedBatch | null;

export function mapStudioBatchDraftListResponse(
  payload: unknown,
): SheinStudioSavedBatch[];

export function normalizeSelectionResponse(
  value: Record<string, unknown> | undefined,
): SDSProductVariantSelection | undefined;

export function normalizeGroupedSelectionsResponse(
  items: Array<Record<string, unknown>> | undefined,
  primarySelection?: SDSProductVariantSelection,
): GroupedSDSSelectionEligibility[];
```

`mapStudioBatchDraftListResponse` deliberately preserves the current permissive
cast and `(response.items ?? [])` behavior; it does not call a new Zod schema.

- [ ] **Step 1: Extend response tests with detail/list fixture equality**

```ts
import {
  sheinStudioBatchDraftDetailContractFixture,
  sheinStudioBatchListContractFixture,
} from "@/lib/api/__fixtures__/shein-studio-batch-contract";
import {
  mapStudioBatchDraftDetailToDraft,
  mapStudioBatchDraftListResponse,
  normalizeGroupedSelectionsResponse,
  normalizeSelectionResponse,
  parseStudioBatchDraftDetailResponse,
} from "@/lib/api/shein-studio-batch-draft-response-codec";

it("maps the established detail domain contract", () => {
  expect(
    mapStudioBatchDraftDetailToDraft(
      parseStudioBatchDraftDetailResponse(
        sheinStudioBatchDraftDetailContractFixture.response,
      ),
    ),
  ).toMatchObject(sheinStudioBatchDraftDetailContractFixture.expectedDraft);
});

it("maps list records and tolerates a missing items property", () => {
  expect(
    mapStudioBatchDraftListResponse(sheinStudioBatchListContractFixture.response),
  ).toMatchObject(sheinStudioBatchListContractFixture.expectedBatches);
  expect(mapStudioBatchDraftListResponse({})).toEqual([]);
});

it("keeps explicit empty list selections and tasks while restoring legacy designs", () => {
  const [batch] = mapStudioBatchDraftListResponse({
    items: [
      {
        id: "batch-1",
        approved_design_ids: [],
        created_tasks: [],
        legacy_compatibility_snapshot: {
          approved_design_ids: ["legacy-design"],
          created_tasks: [{ id: "legacy-task", title: "Legacy task" }],
          designs: [{ id: "legacy-design", image_url: "https://example.com/legacy.png" }],
        },
      },
    ],
  });
  expect(batch).toMatchObject({
    selectedIds: [],
    createdTasks: [],
    designs: [{ id: "legacy-design" }],
  });
});
```

- [ ] **Step 2: Add the detail empty-value precedence matrix test**

Use one mixed record to lock the counter-intuitive rules:

```ts
it("preserves field-specific empty-value legacy precedence", () => {
  const draft = mapStudioBatchDraftDetailToDraft({
    batch: {
      id: "batch-1",
      prompt: "prompt",
      approved_design_ids: [],
      created_tasks: [],
      generation_jobs: [],
      generation_error: "",
      generation_job_id: "",
      legacy_compatibility_snapshot: {
        approved_design_ids: ["legacy-design"],
        created_tasks: [{ id: "legacy-task", title: "Legacy task" }],
        generation_jobs: [{ job_id: "legacy-job", status: "failed" }],
        generation_error: "legacy-error",
        generation_job_id: "legacy-job",
        designs: [{ id: "legacy-design", image_url: "https://example.com/legacy.png" }],
      },
      updated_at: "2026-08-08T00:00:00Z",
    },
    designs: [],
  });

  expect(draft).toMatchObject({
    selectedIds: ["legacy-design"],
    designs: [{ id: "legacy-design" }],
    createdTasks: [],
    generationJobs: [{ jobId: "legacy-job", status: "failed" }],
    generationError: "",
    generationJobId: "",
  });
});

it("treats explicit empty group arrays as canonical", () => {
  const draft = mapStudioBatchDraftDetailToDraft({
    batch: {
      id: "batch-1",
      groups: [
        {
          id: "group-1",
          name: "Group 1",
          primary_selection: {},
          designs: [],
          approved_design_ids: [],
          legacy_compatibility_snapshot: {
            approved_design_ids: ["legacy-design"],
            designs: [{ id: "legacy-design", image_url: "https://example.com/legacy.png" }],
          },
        },
      ],
      updated_at: "2026-08-08T00:00:00Z",
    },
  });

  expect(draft?.groups[0]).toMatchObject({ designs: [], selectedIds: [] });
});

it("preserves omitted hot-style fields and removes the primary grouped selection", () => {
  const draft = mapStudioBatchDraftDetailToDraft({
    batch: { id: "batch-1", updated_at: "2026-08-08T00:00:00Z" },
  });
  expect(draft).not.toHaveProperty("hotStyleReferenceImageUrls");
  expect(draft).not.toHaveProperty("hotStyleReferenceBrief");
  expect(draft).not.toHaveProperty("hotStyleReferencePrompt");

  const primary = normalizeSelectionResponse({ variant_id: 1 });
  expect(
    normalizeGroupedSelectionsResponse(
      [
        { selection: { variant_id: 1 } },
        { selection: { variant_id: 2 } },
      ],
      primary,
    ),
  ).toHaveLength(1);
});
```

- [ ] **Step 3: Run the response test and observe missing mapper failures**

```powershell
npm.cmd test -- src/lib/api/shein-studio-batch-draft-response-codec.test.ts
```

Expected: FAIL because the mapping exports have not moved yet.

- [ ] **Step 4: Move all response composition into the response codec**

Move, without rewriting behavior:

- hot-style property-presence preservation;
- detail-to-draft and detail-to-batch mapping;
- list-item mapping and list response mapping;
- selection and grouped-selection normalization;
- prompt-history and group normalization;
- local scalar/array parsing helpers.

Use Task 1 primitives for batch name, hot-style URL, task, and job normalization,
and for the design records shared by legacy and nested-group paths. Preserve the
dedicated top-level detail design mapping because it also applies batch-level
fallbacks and background-removal fields. Use Task 2 only for legacy snapshot
conversion. Keep `normalizeDraft` and selected-SDS-image normalization in the
response codec.

- [ ] **Step 5: Migrate direct pure-function callers**

In `shein-studio-batches.ts`, import `normalizeSelectionResponse` and
`normalizeGroupedSelectionsResponse` directly from the response codec. In
`shein-studio.test.ts`, import detail mapping functions directly from the
response codec while list/upsert remain imported from transport.

Do not add exports for these functions back to transport.

- [ ] **Step 6: Reduce transport to list/upsert/delete**

The final transport flow must read as:

```ts
export async function listSheinStudioBatchDrafts(options?: StudioBatchDraftRequestOptions) {
  const payload = await apiRequest<unknown>("/studio/batches", listRequestOptions(options));
  return mapStudioBatchDraftListResponse(payload);
}

export async function upsertSheinStudioBatchDraft(
  input: UpsertSheinStudioBatchDraftInput,
  options?: StudioBatchDraftRequestOptions,
) {
  const detail = parseStudioBatchDraftDetailResponse(
    await apiRequest<unknown>("/studio/batches", {
      method: "POST",
      body: buildStudioBatchDraftUpsertPayload(input),
      signal: options?.signal,
      timeoutMs: options?.timeoutMs ?? STUDIO_BATCH_DRAFT_TIMEOUT_MS,
    }),
  );
  return mapStudioBatchDraftDetailToBatch(detail);
}
```

Keep the existing inline list option object if extracting `listRequestOptions`
does not reduce code; the required property is that transport contains no DTO
mapping or legacy access.

- [ ] **Step 7: Run all focused codec, API, and batch tests**

```powershell
npm.cmd test -- `
  src/lib/api/shein-studio-batch-draft-codec-primitives.test.ts `
  src/lib/api/shein-studio-batch-draft-legacy-adapter.test.ts `
  src/lib/api/shein-studio-batch-draft-request-codec.test.ts `
  src/lib/api/shein-studio-batch-draft-response-codec.test.ts `
  src/lib/api/shein-studio.test.ts `
  src/lib/api/shein-studio-batches.test.ts `
  src/lib/utils/shein-studio-batches.test.ts
npm.cmd run typecheck
```

Expected: all focused suites and typecheck PASS.

- [ ] **Step 8: Commit the response mapping and thin transport**

```powershell
git add -- `
  web/listingkit-ui/src/lib/api/shein-studio-batch-draft-response-codec.ts `
  web/listingkit-ui/src/lib/api/shein-studio-batch-draft-response-codec.test.ts `
  web/listingkit-ui/src/lib/api/shein-studio-batch-drafts.ts `
  web/listingkit-ui/src/lib/api/shein-studio-batches.ts `
  web/listingkit-ui/src/lib/api/shein-studio.test.ts
git diff --cached --check
git diff --cached
git commit -m "refactor: split batch draft response codec"
```

---

### Task 6: Audit dependencies and run full regression gates

**Files:**

- Verify: all files changed in Tasks 1-5
- Modify only if a gate exposes a defect: the owning source/test file from Tasks 1-5

**Interfaces:**

- Consumes: the complete codec split from Tasks 1-5.
- Produces: verified acyclic imports, a clean worktree, and evidence for PR readiness.

- [ ] **Step 1: Prove the old schema and facade imports are gone**

Run from the repository root:

```powershell
$oldSchema = rg -n -F 'shein-studio-batch-draft-schema' web/listingkit-ui/src
if ($LASTEXITCODE -eq 0) { $oldSchema; throw 'old schema imports remain' }

$legacyImports = rg -n -F 'shein-studio-batch-draft-legacy-adapter' `
  web/listingkit-ui/src `
  --glob '!lib/api/shein-studio-batch-draft-request-codec.ts' `
  --glob '!lib/api/shein-studio-batch-draft-response-codec.ts' `
  --glob '!lib/api/shein-studio-batch-draft-legacy-adapter.ts' `
  --glob '!lib/api/shein-studio-batch-draft-legacy-adapter.test.ts'
if ($LASTEXITCODE -eq 0) { $legacyImports; throw 'unauthorized legacy adapter import remains' }

$primitiveImports = rg -n -F 'shein-studio-batch-draft-codec-primitives' `
  web/listingkit-ui/src `
  --glob '!lib/api/shein-studio-batch-draft-request-codec.ts' `
  --glob '!lib/api/shein-studio-batch-draft-response-codec.ts' `
  --glob '!lib/api/shein-studio-batch-draft-legacy-adapter.ts' `
  --glob '!lib/api/shein-studio-batch-draft-codec-primitives.ts' `
  --glob '!lib/api/shein-studio-batch-draft-codec-primitives.test.ts'
if ($LASTEXITCODE -eq 0) { $primitiveImports; throw 'unauthorized codec primitive import remains' }
```

Expected: no matches and no thrown errors.

- [ ] **Step 2: Prove transport no longer owns or re-exports pure codec logic**

```powershell
$transport='web/listingkit-ui/src/lib/api/shein-studio-batch-drafts.ts'
$forbidden = rg -n `
  'export function (buildStudioBatchDraftSelectionKey|mapStudioBatchDraftDetailToDraft|mapStudioBatchDraftDetailToBatch|normalizeSelectionResponse|normalizeGroupedSelectionsResponse)|legacyCompatibilitySnapshotToPayload|normalizeLegacyCompatibilitySnapshotResponse|function selectionToPayload|function groupedWorkspaceToPayload' `
  $transport
if ($LASTEXITCODE -eq 0) { $forbidden; throw 'transport still owns codec logic' }
```

Expected: no matches.

- [ ] **Step 3: Run complete frontend verification**

Run from `web/listingkit-ui`:

```powershell
npm.cmd run lint
npm.cmd run typecheck
npm.cmd test
npm.cmd run build
```

Expected: all four commands exit 0. Record any existing non-fatal dependency or
allow-scripts warning separately; do not claim it was introduced by this PR.

- [ ] **Step 4: Run repository-level Go regression tests**

Run from the repository root:

```powershell
$env:GOWORK='off'
go test ./... -count=1
```

Expected: exit 0 for every Go package.

- [ ] **Step 5: Review final scope and worktree state**

```powershell
git diff master...HEAD --stat
git diff master...HEAD -- `
  web/listingkit-ui/src/lib/api `
  web/listingkit-ui/src/lib/utils/shein-studio-batches.ts `
  web/listingkit-ui/src/lib/utils/shein-studio-batches.test.ts
git status --short --branch
```

Expected: only the approved spec/plan and Task 1-5 files appear; the worktree is
clean. If verification required a code correction, rerun that task's focused
test, stage only its owning files, review the cached diff, and commit with a
specific `fix:` message before repeating Steps 1-5.
