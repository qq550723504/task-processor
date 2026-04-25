import {
  deriveTaskPreviewEmptyState,
  deriveTaskQueueEmptyState,
  shouldSuppressResolvedActionSummary,
} from "@/components/listingkit/tasks/task-status-display";

describe("task-status-display", () => {
  it("suppresses the resolved action summary for failed tasks without usable content", () => {
    expect(
      shouldSuppressResolvedActionSummary(
        {
          status: "failed",
        },
        { hasPreviewSvg: false, queueTotal: 0 },
      ),
    ).toBe(true);
  });

  it("keeps the resolved action summary when preview content exists", () => {
    expect(
      shouldSuppressResolvedActionSummary(
        {
          status: "failed",
        },
        { hasPreviewSvg: true, queueTotal: 0 },
      ),
    ).toBe(false);
  });

  it("derives a failure-aware preview empty state", () => {
    expect(
      deriveTaskPreviewEmptyState({
        status: "failed",
        error: "product enrichment failed",
      }),
    ).toEqual({
      title: "Preview unavailable",
      description:
        "product enrichment failed",
    });
  });

  it("derives a queue empty state from task failure details", () => {
    expect(
      deriveTaskQueueEmptyState({
        status: "failed",
        error: "quality score too low",
      }),
    ).toEqual({
      title: "No generation queue items",
      description:
        "quality score too low",
    });
  });
});
