import { describe, expect, it } from "vitest";

import {
  parseDispatchResponse,
  parsePreviewResponse,
  parseReviewPreviewResponse,
  parseTaskResultResponse,
} from "@/lib/api/listingkit-response-schema";

describe("listingkit response schemas", () => {
  it("parses task result responses and rejects invalid task ids", () => {
    expect(
      parseTaskResultResponse({
        task_id: "task-1",
        status: "completed",
        result: { task_id: "task-1", review_reasons: [] },
      }),
    ).toMatchObject({ task_id: "task-1" });

    expect(() =>
      parseTaskResultResponse({ task_id: 123, status: "completed" }),
    ).toThrow("unexpected task result response");
  });

  it("parses preview responses and requires the basic preview shape", () => {
    expect(
      parsePreviewResponse({
        task_id: "task-1",
        status: "completed",
        asset_render_previews: [{ slot: "front" }],
      }),
    ).toMatchObject({ task_id: "task-1" });

    expect(() => parsePreviewResponse({ status: "completed" })).toThrow(
      "unexpected preview response",
    );
  });

  it("parses review preview responses", () => {
    expect(
      parseReviewPreviewResponse({
        task_id: "task-1",
        preview: { slot: "front" },
        revision_status: "fresh",
      }),
    ).toMatchObject({ task_id: "task-1" });

    expect(() =>
      parseReviewPreviewResponse({ preview: "front" }),
    ).toThrow("unexpected review preview response");
  });

  it("parses dispatch responses", () => {
    expect(
      parseDispatchResponse({
        dispatch_kind: "review_preview",
        review_preview: { task_id: "task-1", preview: { slot: "front" } },
      }),
    ).toMatchObject({ dispatch_kind: "review_preview" });

    expect(() =>
      parseDispatchResponse({ dispatch_kind: 123 }),
    ).toThrow("unexpected dispatch response");
  });
});
