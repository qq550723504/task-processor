"use client";

import { useEffect, useState } from "react";

import type { SDSProductVariantSelection } from "@/lib/types/sds";
import {
  loadRecentSDSVariants,
  subscribeRecentSDSVariants,
} from "@/lib/utils/sds-recent-variants";

const emptyRecentVariants: SDSProductVariantSelection[] = [];

export function useSDSRecentVariants() {
  const [items, setItems] =
    useState<SDSProductVariantSelection[]>(emptyRecentVariants);

  useEffect(() => {
    const sync = () => {
      setItems(loadRecentSDSVariants());
    };

    queueMicrotask(sync);
    return subscribeRecentSDSVariants(sync);
  }, []);

  return items;
}
