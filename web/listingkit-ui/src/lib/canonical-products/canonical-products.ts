import { getListingKitTasks } from "@/lib/api/task-list";
import { getListingKitTaskResult } from "@/lib/api/task-result";
import type {
  CanonicalFieldTrace,
  CanonicalProduct,
  ListingKitTaskListQuery,
  ListingKitTaskResult,
} from "@/lib/types/listingkit";

export type CanonicalProductListItem = {
  taskId: string;
  tenantId?: string;
  title: string;
  brand?: string;
  categoryPath: string[];
  imageUrl?: string;
  platformLabels: string[];
  needsReview: boolean;
  imageCount: number;
  variantCount: number;
  createdAt?: string;
  completedAt?: string;
};

export type CanonicalProductDetail = {
  taskId: string;
  tenantId?: string;
  product: CanonicalProduct;
  summary: CanonicalProductListItem;
  reviewFieldCount: number;
  trustedFieldCount: number;
  fieldTraces: Array<{
    field: string;
    trace: CanonicalFieldTrace;
  }>;
};

export type CanonicalProductListPage = {
  page: number;
  pageSize: number;
  items: CanonicalProductListItem[];
};

export async function getCanonicalProducts(
  query: Pick<ListingKitTaskListQuery, "page" | "page_size"> = {},
): Promise<CanonicalProductListPage> {
  const page = query.page && query.page > 0 ? query.page : 1;
  const pageSize = query.page_size && query.page_size > 0 ? query.page_size : 20;
  const tasks = await getListingKitTasks({ page, page_size: pageSize });
  const results = await Promise.allSettled(
    (tasks.items ?? []).map((task) => getListingKitTaskResult(task.task_id)),
  );
  const items = results
    .flatMap((result) => (result.status === "fulfilled" ? [result.value] : []))
    .map(buildCanonicalProductListItem)
    .filter((item): item is CanonicalProductListItem => item !== null);

  return {
    page,
    pageSize,
    items,
  };
}

export async function getCanonicalProductDetail(
  taskId: string,
): Promise<CanonicalProductDetail | null> {
  const result = await getListingKitTaskResult(taskId);
  return buildCanonicalProductDetail(result);
}

export function buildCanonicalProductListItem(
  result: ListingKitTaskResult,
): CanonicalProductListItem | null {
  const product = result.result?.canonical_product;
  if (!product) {
    return null;
  }
  return {
    taskId: result.task_id ?? result.result?.task_id ?? "",
    tenantId: result.tenant_id ?? result.result?.tenant_id,
    title: product.title?.trim() || result.task_id || "Untitled canonical product",
    brand: product.brand,
    categoryPath: product.category_path ?? [],
    imageUrl: product.images?.find((image) => image.url)?.url,
    platformLabels: result.result?.platforms ?? [],
    needsReview: Boolean(product.needs_review),
    imageCount: product.images?.filter((image) => image.url).length ?? 0,
    variantCount: product.variants?.length ?? 0,
    createdAt: result.created_at,
    completedAt: result.completed_at,
  };
}

export function buildCanonicalProductDetail(
  result: ListingKitTaskResult,
): CanonicalProductDetail | null {
  const product = result.result?.canonical_product;
  const summary = buildCanonicalProductListItem(result);
  if (!product || !summary) {
    return null;
  }
  const fieldTraces = Object.entries(product.field_traces ?? {}).map(
    ([field, trace]) => ({ field, trace }),
  );
  return {
    taskId: summary.taskId,
    tenantId: summary.tenantId,
    product,
    summary,
    fieldTraces,
    reviewFieldCount: fieldTraces.filter((item) => item.trace.needs_review).length,
    trustedFieldCount: fieldTraces.filter(
      (item) => !item.trace.needs_review && (item.trace.confidence ?? 0) >= 0.8,
    ).length,
  };
}
