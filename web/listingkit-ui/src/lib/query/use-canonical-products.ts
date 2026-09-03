"use client";

import { useQuery } from "@tanstack/react-query";

import {
  getCanonicalProductDetail,
  getCanonicalProducts,
  type CanonicalProductListPage,
} from "@/lib/api/canonical-products";

async function getAllCanonicalProducts(query: { page?: number; page_size?: number }): Promise<CanonicalProductListPage> {
  const pageSize = query.page_size ?? 100;
  const first = await getCanonicalProducts({ ...query, page: 1, page_size: pageSize });
  const pages = [first];
  for (let page = 2; (page - 1) * pageSize < first.total; page += 1) {
    pages.push(await getCanonicalProducts({ ...query, page, page_size: pageSize }));
  }
  const seen = new Set<string>();
  const items = pages.flatMap((result) => result.items).filter((item) => {
    const key = item.productKey || item.taskId;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
  return { ...first, items, total: items.length };
}

export function useCanonicalProducts(query: { page?: number; page_size?: number }) {
  return useQuery({
    queryKey: ["listingkit", "canonical-products", query],
    queryFn: () => getAllCanonicalProducts(query),
    refetchInterval: 15000,
    refetchOnWindowFocus: true,
  });
}

export function useCanonicalProductDetail(taskId: string) {
  return useQuery({
    queryKey: ["listingkit", "canonical-products", taskId],
    queryFn: () => getCanonicalProductDetail(taskId),
    enabled: Boolean(taskId),
    refetchOnWindowFocus: true,
  });
}
