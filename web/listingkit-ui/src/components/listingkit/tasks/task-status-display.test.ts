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
      title: "预览暂不可用",
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
      title: "暂时没有可处理的队列项",
      description:
        "quality score too low",
    });
  });

  it("prioritizes blocking workflow issue details over legacy child task errors", () => {
    const task = {
      status: "failed",
      result: {
        child_tasks: [
          {
            kind: "product_enrich",
            status: "failed",
            error: "legacy product child task failed",
          },
        ],
        workflow_issues: [
          {
            code: "product_enrich_failed",
            severity: "blocking" as const,
            stage: "product_enrich",
            message: "Product enrichment failed",
            detail: "structured enrichment failure detail",
          },
        ],
      },
    };

    expect(deriveTaskPreviewEmptyState(task)?.description).toBe(
      "structured enrichment failure detail",
    );
    expect(deriveTaskQueueEmptyState(task)?.description).toBe(
      "structured enrichment failure detail",
    );
  });

  it("treats queued and running statuses as in-flight aliases", () => {
    expect(
      deriveTaskPreviewEmptyState({
        status: "queued",
      }),
    ).toEqual({
      title: "预览还在生成中",
      description:
        "任务仍在处理中，生成完成后这里会自动显示预览内容。你也可以稍后从任务列表继续。",
    });
    expect(
      deriveTaskQueueEmptyState({
        status: "running",
      }),
    ).toEqual({
      title: "队列项还在准备中",
      description:
        "任务仍在处理中，生成规划完成后这里会自动出现队列项。你也可以稍后回到任务列表继续。",
    });
  });
});
