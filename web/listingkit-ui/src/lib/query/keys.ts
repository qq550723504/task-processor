import type { ListingKitTaskListQuery, QueueQuery } from "@/lib/types/listingkit";

export type SDSProductsKeyQuery = {
  keyword: string;
  page: number;
  size: number;
  shipmentArea: string;
  categoryId?: number;
  onSaleStatus?: number;
  hotSellStatus?: number;
  sortField?: string;
  sortType?: string;
  weightBand?: string;
  cycleBand?: string;
};

export const listingKitKeys = {
  tasks: (query: ListingKitTaskListQuery) =>
    ["listingkit", "tasks", query] as const,
  preview: (taskId: string) => ["listingkit", taskId, "preview"] as const,
  taskResult: (taskId: string) => ["listingkit", taskId, "task-result"] as const,
  sdsProducts: (query: SDSProductsKeyQuery) =>
    [
      "listingkit",
      "sds",
      "products",
      compactQueryKeyObject(query),
    ] as const,
  sdsShipmentAreas: () => ["listingkit", "sds", "shipment-areas"] as const,
  sdsCategories: (shipmentArea: string) =>
    ["listingkit", "sds", "categories", shipmentArea] as const,
  sdsProductDetail: (productId: number) =>
    ["listingkit", "sds", "product-detail", productId] as const,
  queue: (taskId: string, query: QueueQuery) =>
    ["listingkit", taskId, "queue", query] as const,
  reviewSession: (taskId: string, query: QueueQuery) =>
    ["listingkit", taskId, "review-session", query] as const,
  reviewPreview: (taskId: string, query: QueueQuery) =>
    ["listingkit", taskId, "review-preview", query] as const,
};

function compactQueryKeyObject<T extends Record<string, unknown>>(query: T) {
  return Object.fromEntries(
    Object.entries(query).filter(([, value]) => value !== undefined),
  ) as Partial<T>;
}
