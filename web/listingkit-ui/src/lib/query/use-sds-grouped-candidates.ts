"use client";

import { useEffect, useState } from "react";

import type { SDSProductVariantSelection } from "@/lib/types/sds";
import {
  loadSDSGroupedCandidates,
  subscribeSDSGroupedCandidates,
} from "@/lib/utils/sds-grouped-candidates";

const emptyGroupedCandidates: SDSProductVariantSelection[] = [];

export function useSDSGroupedCandidates() {
  const [items, setItems] =
    useState<SDSProductVariantSelection[]>(emptyGroupedCandidates);

  useEffect(() => {
    const sync = () => {
      setItems(loadSDSGroupedCandidates());
    };

    queueMicrotask(sync);
    return subscribeSDSGroupedCandidates(sync);
  }, []);

  return items;
}
