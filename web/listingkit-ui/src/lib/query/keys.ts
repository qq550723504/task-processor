import type { ListingKitTaskListQuery, QueueQuery } from "@/lib/types/listingkit";

export const listingKitKeys = {
  tasks: (query: ListingKitTaskListQuery) =>
    ["listingkit", "tasks", query] as const,
  preview: (taskId: string) => ["listingkit", taskId, "preview"] as const,
  taskResult: (taskId: string) => ["listingkit", taskId, "task-result"] as const,
  sdsProducts: (
    keyword: string,
    page: number,
    size: number,
    shipmentArea: string,
    categoryId?: number,
    onSaleStatus?: number,
    hotSellStatus?: number,
    sortField?: string,
    sortType?: string,
    weightBand?: string,
    cycleBand?: string,
  ) =>
    [
      "listingkit",
      "sds",
      "products",
      keyword,
      page,
      size,
      shipmentArea,
      categoryId,
      onSaleStatus,
      hotSellStatus,
      sortField,
      sortType,
      weightBand,
      cycleBand,
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
