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

  it("decodes legacy records and filters invalid nested entries", () => {
    expect(
      decodeStudioBatchDraftLegacySnapshot({
        approved_design_ids: ["design-1", 2],
        created_tasks: [
          { id: "task-1", title: "Create", design_id: "design-1" },
          { id: "", title: "Invalid" },
        ],
        generation_jobs: [
          { job_id: "job-1", status: "failed" },
          { job_id: "", status: "running" },
        ],
        designs: [
          { id: "design-1", image_url: "https://example.com/1.png" },
          { image_url: "https://example.com/invalid.png" },
        ],
      }),
    ).toMatchObject({
      selectedIds: ["design-1"],
      createdTasks: [{ id: "task-1", title: "Create", designId: "design-1" }],
      generationJobs: [{ jobId: "job-1", status: "failed" }],
      designs: [{ id: "design-1", imageUrl: "https://example.com/1.png" }],
    });
  });

  it("applies explicit empty overrides without erasing omitted values", () => {
    const legacy = decodeStudioBatchDraftLegacySnapshot({
      approved_design_ids: ["legacy-design"],
      created_tasks: [{ id: "legacy-task", title: "Legacy" }],
      generation_error: "legacy-error",
    });

    expect(
      mergeStudioBatchDraftLegacySnapshot(legacy, {
        createdTasks: [],
        generationError: "",
      }),
    ).toMatchObject({
      selectedIds: ["legacy-design"],
      createdTasks: [],
      generationError: "",
    });
  });
});
