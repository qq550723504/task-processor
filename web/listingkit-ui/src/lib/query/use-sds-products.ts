"use client";

import { useQuery } from "@tanstack/react-query";

import { getSDSProducts } from "@/lib/api/sds-products";
import { listingKitKeys } from "@/lib/query/keys";

export function useSDSProducts({
  keyword,
  page = 1,
  size = 12,
  shipmentArea = "US",
  categoryId,
  onSaleStatus,
  hotSellStatus,
  sortField,
  sortType,
  weightBand,
  cycleBand,
}: {
  keyword: string;
  page?: number;
  size?: number;
  shipmentArea?: string;
  categoryId?: number;
  onSaleStatus?: number;
  hotSellStatus?: number;
  sortField?: string;
  sortType?: string;
  weightBand?: string;
  cycleBand?: string;
}) {
  return useQuery({
    queryKey: listingKitKeys.sdsProducts({
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
    }),
    queryFn: () =>
      getSDSProducts({
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
      }),
  });
}
