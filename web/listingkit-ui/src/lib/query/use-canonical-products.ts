"use client";

import { useQuery } from "@tanstack/react-query";

import {
  getCanonicalProductDetail,
  getCanonicalProducts,
} from "@/lib/api/canonical-products";

export function useCanonicalProducts(query: { page?: number; page_size?: number }) {
  return useQuery({
    queryKey: ["listingkit", "canonical-products", query],
    queryFn: () => getCanonicalProducts(query),
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
