import { beforeEach, describe, expect, it, vi } from "vitest";

import { getListingKitTasks } from "@/lib/api/task-list";

import {
  buildCanonicalProductDetail,
  buildCanonicalProductListItem,
  getCanonicalProducts,
} from "./canonical-products";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

vi.mock("@/lib/api/task-list", () => ({
  getListingKitTasks: vi.fn(),
}));

const mockedGetListingKitTasks = vi.mocked(getListingKitTasks);

const taskResult: ListingKitTaskResult = {
  task_id: "task-1",
  tenant_id: "tenant-a",
  status: "completed",
  source_reference: {
    key: "crawler:1688:888",
    type: "crawler",
    platform: "1688",
    id: "888",
    url: "https://detail.1688.com/offer/888.html",
  },
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

describe("getCanonicalProducts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("uses canonical product summaries from the paginated task list without detail requests", async () => {
    mockedGetListingKitTasks.mockResolvedValueOnce({
      page: 1,
      page_size: 30,
      total: 2,
      items: [
        {
          task_id: "task-1",
          tenant_id: "tenant-a",
          platforms: ["shein"],
          title: "Canvas Tote",
          canonical_product: {
            title: "Canvas Tote",
            brand: "Studio",
            category_path: ["Bags"],
            image_url: "https://example.com/main.jpg",
            image_count: 3,
            variant_count: 2,
            needs_review: false,
          },
          created_at: "2026-05-09T01:00:00Z",
          completed_at: "2026-05-09T01:03:00Z",
        },
      ],
    });

    await expect(getCanonicalProducts({ page: 1, page_size: 30 })).resolves.toMatchObject({
      page: 1,
      pageSize: 30,
      items: [
        expect.objectContaining({
          taskId: "task-1",
          title: "Canvas Tote",
          imageCount: 3,
          variantCount: 2,
        }),
      ],
    });
    expect(mockedGetListingKitTasks).toHaveBeenCalledWith({
      page: 1,
      page_size: 30,
      canonical_product: true,
    });
  });
});

describe("buildCanonicalProductDetail", () => {
  it("keeps review trace counts and raw product fields for detail view", () => {
    const detail = buildCanonicalProductDetail(taskResult);

    expect(detail?.reviewFieldCount).toBe(1);
    expect(detail?.trustedFieldCount).toBe(1);
    expect(detail?.product.title).toBe("Bluetooth Headphones");
    expect(detail?.summary?.variantCount).toBe(2);
    expect(detail?.sourceReference).toEqual({
      key: "crawler:1688:888",
      type: "crawler",
      platform: "1688",
      id: "888",
      url: "https://detail.1688.com/offer/888.html",
    });
    expect(detail?.sourceReference).not.toBe(taskResult.source_reference);
  });

  it("omits source lineage for legacy task results", () => {
    const detail = buildCanonicalProductDetail({
      ...taskResult,
      source_reference: undefined,
    });

    expect(detail?.sourceReference).toBeUndefined();
  });
});
