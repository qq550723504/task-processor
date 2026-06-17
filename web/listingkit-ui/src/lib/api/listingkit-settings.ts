import { apiRequest } from "@/lib/api/client";
import type {
  ListingKitSettingsHealthPage,
  ListingKitSettingsNamespaceListResponse,
  ListingKitSettingsNamespaceSchema,
} from "@/lib/types/listingkit";

type SettingsQuery = Record<string, string | undefined>;

export function listListingKitSettingsNamespaces() {
  return apiRequest<ListingKitSettingsNamespaceListResponse>("/settings");
}

export function getListingKitSettingsHealth() {
  return apiRequest<ListingKitSettingsHealthPage>("/settings-health");
}

export function getListingKitSettingsSchema(namespace: string) {
  return apiRequest<ListingKitSettingsNamespaceSchema>(
    `/settings/${encodeURIComponent(namespace)}/schema`,
  );
}

export function getListingKitSettings<T>(
  namespace: string,
  query?: SettingsQuery,
) {
  return apiRequest<T>(`/settings/${encodeURIComponent(namespace)}`, {
    query,
  });
}

export function updateListingKitSettings<T>(
  namespace: string,
  body: unknown,
  query?: SettingsQuery,
) {
  return apiRequest<T>(`/settings/${encodeURIComponent(namespace)}`, {
    method: "PUT",
    query,
    body,
  });
}
