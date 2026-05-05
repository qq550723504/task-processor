import {
  presentPlatformStatus,
  presentQueueReviewStatus,
  presentQueueState,
  presentTaskStatus,
} from "@/components/listingkit/shared/status-presentation";

describe("status-presentation", () => {
  it("maps failed task status to a danger presentation", () => {
    expect(presentTaskStatus("failed")).toEqual({
      label: "失败",
      title: "任务处理失败",
      tone: "danger",
    });
  });

  it("maps processing task status to an in-progress presentation", () => {
    expect(presentTaskStatus("processing")).toEqual({
      label: "处理中",
      title: "任务处理中",
      tone: "warning",
    });
  });

  it("maps review-ready platform status to warning review copy", () => {
    expect(presentPlatformStatus({ status: "review_ready", needs_review: true })).toEqual({
      label: "待检查",
      tone: "warning",
    });
  });

  it("maps retry-needed platform status to danger retry copy", () => {
    expect(presentPlatformStatus({ status: "retry_needed", needs_review: false })).toEqual({
      label: "需要重试",
      tone: "danger",
    });
  });

  it("maps queue state to clearer copy", () => {
    expect(presentQueueState({ state: "fallback" })).toEqual({
      label: "使用兜底结果",
      tone: "warning",
    });
  });

  it("maps pending review queue status to review copy", () => {
    expect(presentQueueReviewStatus({ review_status: "pending" })).toEqual({
      label: "待复核",
      tone: "warning",
    });
  });
});
