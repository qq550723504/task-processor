import { getListingKitTasks } from "@/lib/api/task-list";
import { buildTaskWorkspaceHref } from "@/lib/listingkit/task-workspace-href";
import { getListingKitTaskResult } from "@/lib/api/task-result";
import type {
  CanonicalFieldTrace,
  CanonicalProduct,
  ListingKitSourceReference,
  ListingKitTaskListQuery,
  ListingKitTaskListItem,
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
  sourceReference?: ListingKitSourceReference;
  workspaceHref: string;
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
  total: number;
  items: CanonicalProductListItem[];
};

export async function getCanonicalProducts(
  query: Pick<ListingKitTaskListQuery, "page" | "page_size"> = {},
): Promise<CanonicalProductListPage> {
  const page = query.page && query.page > 0 ? query.page : 1;
  const pageSize = query.page_size && query.page_size > 0 ? query.page_size : 20;
  const tasks = await getListingKitTasks({
    page,
    page_size: pageSize,
    canonical_product: true,
  });
  const items = (tasks.items ?? [])
    .map(buildCanonicalProductListItemFromTask)
    .filter((item): item is CanonicalProductListItem => item !== null);

  return {
    page,
    pageSize,
    total: tasks.total,
    items,
  };
}

 function buildCanonicalProductListItemFromTask(
  task: ListingKitTaskListItem,
): CanonicalProductListItem | null {
  const product = task.canonical_product;
  if (!product) {
    return null;
  }
  return {
    taskId: task.task_id,
    tenantId: task.tenant_id,
    title: product.title?.trim() || task.title?.trim() || task.task_id || "Untitled canonical product",
    brand: product.brand,
    categoryPath: product.category_path ?? [],
    imageUrl: product.image_url,
    platformLabels: task.platforms ?? [],
    needsReview: Boolean(product.needs_review),
    imageCount: product.image_count ?? task.image_count ?? 0,
    variantCount: product.variant_count ?? 0,
    createdAt: task.created_at,
    completedAt: task.completed_at,
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
    sourceReference: result.source_reference
      ? { ...result.source_reference }
      : undefined,
    workspaceHref: buildTaskWorkspaceHref({
      task_id: summary.taskId,
      platforms: result.result?.platforms,
      shein_workflow_status: result.shein_workflow_status,
    }),
    product,
    summary,
    fieldTraces,
    reviewFieldCount: fieldTraces.filter((item) => item.trace.needs_review).length,
    trustedFieldCount: fieldTraces.filter(
      (item) => !item.trace.needs_review && (item.trace.confidence ?? 0) >= 0.8,
    ).length,
  };
}
