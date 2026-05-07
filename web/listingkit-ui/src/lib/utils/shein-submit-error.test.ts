import { describe, expect, it } from "vitest";

import { ApiError } from "@/lib/api/client";
import { formatSheinSubmitError } from "@/lib/utils/shein-submit-error";

describe("formatSheinSubmitError", () => {
  it("keeps explicit API payload messages", () => {
    const error = new ApiError("ListingKit API request failed: 400", 400, {
      message: "SHEIN image upload unavailable",
    });

    expect(formatSheinSubmitError(error)).toBe("SHEIN image upload unavailable");
  });

  it("rewrites readiness-blocked submit failures into customer-facing guidance", () => {
    const error = new ApiError("ListingKit API request failed: 400", 400, {
      message: "submit blocked by readiness: 当前仍有关键字段未完成，SHEIN 资料包还不能直接进入提交态",
    });

    expect(
      formatSheinSubmitError(error, {
        submit_readiness: {
          ready: false,
          blocking_items: [
            {
              key: "final_images",
              label: "最终图片",
              message: "缺少尺寸图标记，请在 SHEIN data images 中标记一张尺寸图",
            },
          ],
        },
      }),
    ).toBe("提交前检查未通过：缺少尺寸图标记，请在 SHEIN data images 中标记一张尺寸图");
  });

  it("rewrites in-flight submit conflicts into wait guidance", () => {
    const error = new ApiError("ListingKit API request failed: 409", 409, {
      message: "submit already in progress: shein publish is in submit_remote",
    });

    expect(formatSheinSubmitError(error)).toBe(
      "已有 SHEIN 提交正在进行，请等待当前提交完成或刷新状态后再操作。",
    );
  });
});
