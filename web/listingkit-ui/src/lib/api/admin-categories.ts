import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

export const listingCategorySchema: z.ZodType<{
  id: number;
  tenantId?: number;
  name: string;
  code: string;
  parentId: number;
  level: number;
  sort: number;
  icon?: string;
  image?: string;
  description?: string;
  status: number;
  createTime?: string;
  updateTime?: string;
  children?: ListingCategory[];
}> = z.lazy(() =>
  z
    .object({
      id: z.number(),
      tenantId: z.number().optional(),
      name: z.string(),
      code: z.string(),
      parentId: z.number(),
      level: z.number(),
      sort: z.number(),
      icon: z.string().optional(),
      image: z.string().optional(),
      description: z.string().optional(),
      status: z.number(),
      createTime: z.string().optional(),
      updateTime: z.string().optional(),
      children: z.array(listingCategorySchema).optional(),
    })
    .passthrough(),
);

const categoryListSchema = z.array(listingCategorySchema);

export type ListingCategory = z.infer<typeof listingCategorySchema>;

export type ListingCategoryQuery = Omit<QueueQuery, "status"> & {
  name?: string;
  code?: string;
  parentId?: number;
  level?: number;
  status?: string;
};

export type ListingCategoryInput = {
  name: string;
  code: string;
  parentId?: number;
  level?: number;
  sort?: number;
  icon?: string;
  image?: string;
  description?: string;
  status?: number;
};

export function parseCategoryListResponse(
  payload: unknown,
): ListingCategory[] {
  return parseApiResponseShape(
    payload,
    categoryListSchema,
    "ListingKit API returned an unexpected category list response",
  );
}

export function parseCategoryResponse(payload: unknown): ListingCategory {
  return parseApiResponseShape(
    payload,
    listingCategorySchema,
    "ListingKit API returned an unexpected category response",
  );
}

export async function getListingCategories(
  query: ListingCategoryQuery = {},
): Promise<ListingCategory[]> {
  const payload = await apiRequest<unknown>("/admin/categories", { query });
  return parseCategoryListResponse(payload);
}

export async function createListingCategory(
  input: ListingCategoryInput,
): Promise<ListingCategory> {
  const payload = await apiRequest<unknown>("/admin/categories", {
    method: "POST",
    body: input,
  });
  return parseCategoryResponse(payload);
}

export async function updateListingCategoryStatus(
  id: number,
  status: number,
): Promise<ListingCategory> {
  const payload = await apiRequest<unknown>(`/admin/categories/${id}/status`, {
    method: "PATCH",
    body: { status },
  });
  return parseCategoryResponse(payload);
}

export async function deleteListingCategory(id: number): Promise<void> {
  await apiRequest<unknown>(`/admin/categories/${id}`, { method: "DELETE" });
}
