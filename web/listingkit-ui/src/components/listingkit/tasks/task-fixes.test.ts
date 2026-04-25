import { describe, expect, it } from "vitest";

import {
  extractTaskFixes,
  inferTaskDraftFocus,
  inferTaskDraftFocusFromDraft,
} from "@/components/listingkit/tasks/task-fixes";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

describe("task fixes", () => {
  it("focuses product URL when fixes mention a source link", () => {
    const task = {
      task_id: "task_123",
      status: "failed",
      created_at: "2026-04-19T00:00:00Z",
      error: "1. 添加可访问的 1688 商品链接\n2. 补充更多产品信息",
    } satisfies ListingKitTaskResult;

    expect(extractTaskFixes(task)).toEqual([
      "添加可访问的 1688 商品链接",
      "补充更多产品信息",
    ]);
    expect(inferTaskDraftFocus(task)).toBe("productUrl");
  });

  it("prefers product URL focus for URL-only drafts", () => {
    expect(
      inferTaskDraftFocusFromDraft({
        text: "",
        imageUrls: "",
        productUrl: "https://detail.1688.com/offer/123456789.html",
        platforms: ["shein"],
      }),
    ).toBe("productUrl");
  });
});
