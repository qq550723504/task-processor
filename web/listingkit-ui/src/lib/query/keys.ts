import type { ListingKitTaskListQuery, QueueQuery } from "@/lib/types/listingkit";
import type { RevisionHistoryQuery } from "@/lib/api/revision-history";

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
  storeProfiles: () => ["listingkit", "store-profiles"] as const,
  sheinEnrollmentDashboard: (query: { activity_type?: string }) =>
    [
      "listingkit",
      "shein-enrollment",
      "dashboard",
      compactQueryKeyObject(query),
    ] as const,
  sheinEnrollmentStoreSummary: (
    storeId: number,
    query: { activity_type?: string },
  ) =>
    [
      "listingkit",
      "shein-enrollment",
      storeId,
      "summary",
      compactQueryKeyObject(query),
    ] as const,
  sheinEnrollmentProducts: (
    storeId: number,
    query: {
      skc_name?: string;
      is_active?: boolean;
      page?: number;
      page_size?: number;
    },
  ) =>
    [
      "listingkit",
      "shein-enrollment",
      storeId,
      "products",
      compactQueryKeyObject(query),
    ] as const,
  sheinEnrollmentSDSCostGroups: (
    storeId: number,
    query: {
      page?: number;
      page_size?: number;
    },
  ) =>
    [
      "listingkit",
      "shein-enrollment",
      storeId,
      "sds-cost-groups",
      compactQueryKeyObject(query),
    ] as const,
  sheinEnrollmentCandidates: (
    storeId: number,
    query: {
      activity_type: string;
      activity_key?: string;
      skc_name?: string;
      candidate_version?: string;
      page?: number;
      page_size?: number;
    },
  ) =>
    [
      "listingkit",
      "shein-enrollment",
      storeId,
      "candidates",
      compactQueryKeyObject(query),
    ] as const,
  sheinEnrollmentRuns: (
    storeId: number,
    query: {
      activity_type?: string;
      activity_key?: string;
      page?: number;
      page_size?: number;
    },
  ) =>
    [
      "listingkit",
      "shein-enrollment",
      storeId,
      "runs",
      compactQueryKeyObject(query),
    ] as const,
  sheinEnrollmentStoreScope: (storeId: number) =>
    ["listingkit", "shein-enrollment", storeId] as const,
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
  revisionHistory: (taskId: string, query: RevisionHistoryQuery) =>
    ["listingkit", taskId, "revision-history", compactQueryKeyObject(query)] as const,
  revisionHistoryDetail: (taskId: string, revisionId: string, compareTo?: string) =>
    ["listingkit", taskId, "revision-history-detail", revisionId, compareTo ?? ""] as const,
};

function compactQueryKeyObject<T extends Record<string, unknown>>(query: T) {
  return Object.fromEntries(
    Object.entries(query).filter(([, value]) => value !== undefined),
  ) as Partial<T>;
}
