"use client";

import { useQuery } from "@tanstack/react-query";

import {
  getCanonicalProductDetail,
  getCanonicalProducts,
} from "@/lib/api/canonical-products";
import type { CanonicalProductListPage } from "@/lib/canonical-products/canonical-products";

async function getCanonicalProductsPage(query: { page?: number; page_size?: number }): Promise<CanonicalProductListPage> {
  return getCanonicalProducts(query);
}

export function useCanonicalProducts(query: { page?: number; page_size?: number }) {
  return useQuery({
    queryKey: ["listingkit", "canonical-products", query],
    queryFn: () => getCanonicalProductsPage(query),
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
