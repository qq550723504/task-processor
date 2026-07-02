import { apiRequest } from "@/lib/api/client";
import type {
  SheinPODImageLookupQuery,
  SheinPODImageLookupResponse,
} from "@/lib/types/listingkit/shein-pod-image-lookup";

export async function lookupSheinPODImages(
  storeId: number,
  query: SheinPODImageLookupQuery,
): Promise<SheinPODImageLookupResponse> {
  return apiRequest<SheinPODImageLookupResponse>(
    `/shein-pod-image-lookup/stores/${storeId}`,
    { query },
  );
}
