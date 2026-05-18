import {
  type ListingStore,
  type ListingStoreInput,
  type ListingStorePage,
  type ListingStoreQuery,
  parseSimpleStoreResponse,
  parseStorePageResponse,
  parseStoreResponse,
  type SimpleListingStore,
} from "@/lib/api/admin-stores";
import { apiRequest } from "@/lib/api/client";

export async function getTenantListingStores(
  query: ListingStoreQuery = {},
): Promise<ListingStorePage> {
  const payload = await apiRequest<unknown>("/stores", { query });
  return parseStorePageResponse(payload);
}

export async function getSimpleTenantListingStores(): Promise<SimpleListingStore[]> {
  const payload = await apiRequest<unknown>("/stores/simple");
  return parseSimpleStoreResponse(payload);
}

export async function createTenantListingStore(
  input: ListingStoreInput,
): Promise<ListingStore> {
  const payload = await apiRequest<unknown>("/stores", {
    method: "POST",
    body: input,
  });
  return parseStoreResponse(payload);
}

export async function updateTenantListingStore(
  id: number,
  input: ListingStoreInput,
): Promise<ListingStore> {
  const payload = await apiRequest<unknown>(`/stores/${id}`, {
    method: "PUT",
    body: input,
  });
  return parseStoreResponse(payload);
}

export async function deleteTenantListingStore(id: number): Promise<void> {
  await apiRequest<unknown>(`/stores/${id}`, { method: "DELETE" });
}
