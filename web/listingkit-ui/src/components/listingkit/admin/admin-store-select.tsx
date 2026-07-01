"use client";

import { useQuery } from "@tanstack/react-query";

import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import {
  getSimpleListingStores,
  type SimpleListingStore,
} from "@/lib/api/admin-stores";

const ADMIN_SIMPLE_STORES_QUERY_KEY = ["listingkit-admin-simple-stores"] as const;

export function useAdminSimpleStores() {
  return useQuery({
    queryKey: ADMIN_SIMPLE_STORES_QUERY_KEY,
    queryFn: getSimpleListingStores,
  });
}

export function formatAdminStoreName(
  stores: Array<Pick<SimpleListingStore, "id" | "name">>,
  storeId: number | undefined,
  emptyLabel = "-",
) {
  if (!storeId) {
    return emptyLabel;
  }
  const store = stores.find((item) => item.id === storeId);
  return store ? formatAdminStoreOption(store) : `#${storeId}`;
}

export function formatAdminStoreOption(
  store: Pick<SimpleListingStore, "id" | "name">,
) {
  const name = store.name.trim() || "未命名店铺";
  return `${name} (#${store.id})`;
}

export function AdminStoreSelect({
  label = "店铺",
  value,
  onChange,
  stores,
  emptyLabel,
  emptyValue = 0,
  filterStore,
}: {
  label?: string;
  value: number | undefined;
  onChange: (storeId: number) => void;
  stores: SimpleListingStore[];
  emptyLabel: string;
  emptyValue?: number;
  filterStore?: (store: SimpleListingStore) => boolean;
}) {
  const visibleStores = filterStore ? stores.filter(filterStore) : stores;

  return (
    <Label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <Select
        value={String(value ?? emptyValue)}
        onChange={(event) => onChange(Number(event.target.value) || emptyValue)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900"
      >
        <option value={emptyValue}>{emptyLabel}</option>
        {visibleStores.map((store) => (
          <option key={store.id} value={store.id}>
            {formatAdminStoreOption(store)}
          </option>
        ))}
      </Select>
    </Label>
  );
}
