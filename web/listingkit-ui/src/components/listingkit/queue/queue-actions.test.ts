import { deriveQueueItemAction } from "@/components/listingkit/queue/queue-actions";

describe("deriveQueueItemAction", () => {
  it("prefers review when a render preview is available", () => {
    const result = deriveQueueItemAction({
      platform: "shein",
      slot: "main",
      render_preview_available: true,
      preview_capabilities: ["detail_preview"],
      retryable: true,
      quality_grade: "provisional",
    });

    expect(result.kind).toBe("review");
    expect(result.label).toBe("Review");
    expect(result.workspaceQuery).toEqual({
      platform: "shein",
      slot: "main",
      preview_capability: "detail_preview",
    });
  });

  it("falls back to retry when preview is unavailable but item is retryable", () => {
    const result = deriveQueueItemAction({
      platform: "temu",
      slot: "gallery",
      render_preview_available: false,
      retryable: true,
      quality_grade: "provisional",
      execution_quality: "failed",
    });

    expect(result.kind).toBe("retry");
    expect(result.label).toBe("Retry");
    expect(result.request).toEqual({
      action_key: "retry_section_generation",
      response_mode: "patch_only",
      target: {
        action_key: "retry_section_generation",
        interaction_mode: "retry_only",
        filters: {
          platforms: ["temu"],
          quality_grade: "provisional",
          retryable_only: true,
          execution_quality: "failed",
        },
      },
    });
  });

  it("uses inspect for non-previewable, non-retryable rows", () => {
    const result = deriveQueueItemAction({
      platform: "walmart",
      slot: "auxiliary",
      render_preview_available: false,
      retryable: false,
    });

    expect(result.kind).toBe("inspect");
    expect(result.label).toBe("Inspect");
    expect(result.workspaceQuery).toEqual({
      platform: "walmart",
      slot: "auxiliary",
    });
  });
});
