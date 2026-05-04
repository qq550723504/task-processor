import {
  presentRecoveryDescriptor,
  presentRetryHint,
} from "@/components/listingkit/shared/hint-presentation";

describe("hint-presentation", () => {
  it("maps retry hints to readable labels", () => {
    expect(presentRetryHint("retry_dispatch")).toEqual({
      label: "重新生成当前内容",
      description: "当前生成步骤可以立即重试。",
    });
  });

  it("maps recovery descriptors to readable content", () => {
    expect(
      presentRecoveryDescriptor({
        recovery_hint: "review_fallback",
        recovery_cta_kind: "review",
        recovery_severity: "medium",
        recovery_urgency: "now",
      }),
    ).toEqual({
      title: "使用兜底结果继续检查",
      description:
        "当前有一份兜底结果可用，建议先检查后再决定是否重试。",
      ctaLabel: "检查恢复项",
      metaLabel: "中优先级 / 立即处理",
    });
  });

  it("falls back cleanly for unknown hints", () => {
    expect(presentRetryHint("custom_hint")).toEqual({
      label: "Custom hint",
      description: "请先按照当前任务提示处理，再决定是否重试。",
    });
  });
});
