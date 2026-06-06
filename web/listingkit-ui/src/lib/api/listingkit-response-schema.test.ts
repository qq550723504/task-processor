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
        result: {
          task_id: "task-1",
          review_reasons: [],
          pod_execution: {
            provider: "sds",
            dependency_mode: "required",
            status: "failed_blocking",
            history: [
              {
                kind: "policy_decision",
                code: "pod_policy_applied",
                occurred_at: "2026-05-28T08:00:00Z",
              },
            ],
          },
        },
      }),
    ).toMatchObject({ task_id: "task-1" });

    expect(
      parseTaskResultResponse({
        task_id: "task-2",
        status: "blocked_retryable",
        retryable_block: {
          reason_code: "worker_queue_backpressure",
          reason_message: "Worker queue is temporarily full.",
          blocked_at: "2026-06-06T04:00:00Z",
          next_retry_at: "2026-06-06T04:15:00Z",
          retry_attempts: 2,
          max_auto_retry_attempts: 5,
          recovery_scope: "task",
          auto_resume_enabled: true,
        },
      }),
    ).toMatchObject({
      task_id: "task-2",
      status: "blocked_retryable",
      retryable_block: {
        reason_code: "worker_queue_backpressure",
        next_retry_at: "2026-06-06T04:15:00Z",
        retry_attempts: 2,
      },
    });

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
