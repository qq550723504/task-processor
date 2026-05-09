import { describe, expect, it } from "vitest";

import {
  buildCanonicalProductDetail,
  buildCanonicalProductListItem,
} from "./canonical-products";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

const taskResult: ListingKitTaskResult = {
  task_id: "task-1",
  tenant_id: "tenant-a",
  status: "completed",
  created_at: "2026-05-09T01:00:00Z",
  completed_at: "2026-05-09T01:03:00Z",
  result: {
    task_id: "task-1",
    platforms: ["shein"],
    canonical_product: {
      title: "Bluetooth Headphones",
      brand: "SoundPeak",
      category_path: ["Electronics", "Audio"],
      needs_review: true,
      images: [{ url: "https://example.com/main.jpg" }],
      variants: [{ sku: "SP-BLK" }, { sku: "SP-WHT" }],
      field_traces: {
        brand: { confidence: 1 },
        material: { needs_review: true, confidence: 0.4 },
      },
    },
  },
};

describe("buildCanonicalProductListItem", () => {
  it("summarizes a task result with a canonical product", () => {
    expect(buildCanonicalProductListItem(taskResult)).toEqual({
      taskId: "task-1",
      tenantId: "tenant-a",
      title: "Bluetooth Headphones",
      brand: "SoundPeak",
      categoryPath: ["Electronics", "Audio"],
      imageUrl: "https://example.com/main.jpg",
      platformLabels: ["shein"],
      needsReview: true,
      imageCount: 1,
      variantCount: 2,
      createdAt: "2026-05-09T01:00:00Z",
      completedAt: "2026-05-09T01:03:00Z",
    });
  });

  it("returns null when the task has no canonical product", () => {
    expect(buildCanonicalProductListItem({ task_id: "task-2" })).toBeNull();
  });
});

describe("buildCanonicalProductDetail", () => {
  it("keeps review trace counts and raw product fields for detail view", () => {
    const detail = buildCanonicalProductDetail(taskResult);

    expect(detail?.reviewFieldCount).toBe(1);
    expect(detail?.trustedFieldCount).toBe(1);
    expect(detail?.product.title).toBe("Bluetooth Headphones");
    expect(detail?.summary?.variantCount).toBe(2);
  });
});
