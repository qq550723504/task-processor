import { describe, expect, it } from "vitest";

import {
  sheinLatestSubmissionSummary,
  sheinSubmitPhaseLabel,
} from "@/lib/shein-studio/shein-submission-display";

describe("shein-submission-display", () => {
  it("labels structured submit phases", () => {
    expect(sheinSubmitPhaseLabel("upload_images")).toBe("上传图片");
    expect(sheinSubmitPhaseLabel("submit_remote")).toBe("提交 SHEIN");
  });

  it("includes failed phase and request id in latest submission summary", () => {
    expect(
      sheinLatestSubmissionSummary({
        last_action: "publish",
        last_status: "failed",
        last_error: "upload rejected",
        publish: {
          action: "publish",
          status: "failed",
          phase: "upload_images",
          request_id: "submit-123",
        },
      }),
    ).toBe("失败阶段：上传图片。Request ID: submit-123。请根据待处理问题修复后再次提交。");
  });
});
