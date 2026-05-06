import { describe, expect, it } from "vitest";

import { extractTaskReviewReasons } from "@/components/listingkit/tasks/task-review-reasons";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

describe("task review reasons", () => {
  it("prefers review and blocking workflow issues over legacy errors", () => {
    const task = {
      status: "needs_review",
      error: "legacy fallback",
      result: {
        workflow_issues: [
          {
            severity: "warning",
            stage: "product_image",
            message: "Image processing used fallback",
          },
          {
            severity: "review",
            stage: "shein_review",
            message: "Confirm SHEIN category",
          },
          {
            severity: "blocking",
            stage: "product_enrich",
            message: "Product enrichment failed",
          },
        ],
      },
    } satisfies ListingKitTaskResult;

    expect(extractTaskReviewReasons(task)).toEqual([
      "Confirm SHEIN category",
      "Product enrichment failed",
    ]);
  });
});
