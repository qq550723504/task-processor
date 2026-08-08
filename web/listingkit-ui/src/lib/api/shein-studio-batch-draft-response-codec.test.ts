import { describe, expect, it } from "vitest";

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
            designs: [
              {
                id: "legacy-design",
                image_url: "https://example.com/legacy.png",
              },
            ],
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
          designs: [
            {
              id: "legacy-design",
              image_url: "https://example.com/legacy.png",
            },
          ],
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
              designs: [
                {
                  id: "legacy-design",
                  image_url: "https://example.com/legacy.png",
                },
              ],
            },
          },
        ],
        updated_at: "2026-08-08T00:00:00Z",
      },
    });

    expect(draft?.groups?.[0]).toMatchObject({ designs: [], selectedIds: [] });
  });

  it("preserves omitted hot-style fields", () => {
    const draft = mapStudioBatchDraftDetailToDraft({
      batch: { id: "batch-1", updated_at: "2026-08-08T00:00:00Z" },
    });

    expect(draft).not.toHaveProperty("hotStyleReferenceImageUrls");
    expect(draft).not.toHaveProperty("hotStyleReferenceBrief");
    expect(draft).not.toHaveProperty("hotStyleReferencePrompt");
  });

  it("removes the primary selection from grouped selections", () => {
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
});
