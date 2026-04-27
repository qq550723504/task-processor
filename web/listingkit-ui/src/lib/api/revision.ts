import { apiRequest } from "@/lib/api/client";
import type {
  ListingKitPreview,
  SheinPendingAttributeCandidate,
  SheinResolvedAttribute,
  SheinSourceAttribute,
} from "@/lib/types/listingkit";

export type ApplyRevisionRequest = {
  platform: "shein";
  actor?: string;
  reason?: string;
  shein?: {
    category_resolution?: {
      category_id?: number;
      category_id_list?: number[];
      product_type_id?: number;
      top_category_id?: number;
      matched_path?: string[];
      status?: string;
      source?: string;
    };
    sale_attribute_resolution?: {
      recommend_category_review?: boolean;
      category_review_reason?: string;
    };
    attribute_resolution?: {
      status?: string;
      source?: string;
      category_id?: number;
      template_count?: number;
      resolved_count?: number;
      unresolved_count?: number;
      resolved_attributes?: SheinResolvedAttribute[];
      pending_attributes?: SheinSourceAttribute[];
      pending_attribute_candidates?: SheinPendingAttributeCandidate[];
      recommended_attribute_candidates?: SheinPendingAttributeCandidate[];
      review_notes?: string[];
    };
  };
};

export function applyRevision(taskId: string, body: ApplyRevisionRequest) {
  return apiRequest<ListingKitPreview>(`/tasks/${taskId}/revision`, {
    method: "POST",
    body,
  });
}
