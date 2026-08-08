import { describe, expect, it } from "vitest";

import {
  deriveStudioBatchDraftName,
  normalizeStudioBatchCreatedTasks,
  normalizeStudioBatchDesignResponse,
  normalizeStudioBatchGenerationJobs,
  normalizeStudioHotStyleReferenceImageUrls,
} from "@/lib/api/shein-studio-batch-draft-codec-primitives";

describe("SHEIN Studio batch draft codec primitives", () => {
  it("trims and bounds derived batch names", () => {
    expect(deriveStudioBatchDraftName("  retro cherries  ")).toBe("retro cherries");
    expect(deriveStudioBatchDraftName("x".repeat(40))).toBe(`${"x".repeat(36)}...`);
  });

  it("keeps only the first unique hot-style reference URL", () => {
    expect(
      normalizeStudioHotStyleReferenceImageUrls([
        "  https://example.com/a.png  ",
        "https://example.com/a.png",
        "https://example.com/b.png",
      ]),
    ).toEqual(["https://example.com/a.png"]);
  });

  it("normalizes snake-case and camel-case generated design fields", () => {
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
  });

  it("filters invalid created tasks and accepts design_id", () => {
    expect(
      normalizeStudioBatchCreatedTasks(
        [{ id: "task-1", title: "Create", design_id: "design-1" }, null],
        [],
        [],
      ),
    ).toEqual([{ id: "task-1", title: "Create", designId: "design-1" }]);
  });

  it("filters blank job IDs and defaults only unknown statuses", () => {
    expect(
      normalizeStudioBatchGenerationJobs([
        { job_id: " job-1 ", status: "succeeded" },
        { job_id: "", status: "failed" },
        { job_id: "job-2", status: "unknown" },
      ]),
    ).toEqual([
      { jobId: "job-1", status: "succeeded" },
      { jobId: "job-2", status: "running" },
    ]);
  });
});
