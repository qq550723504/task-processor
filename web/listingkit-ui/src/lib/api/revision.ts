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
    regenerate_attributes?: boolean;
    regenerate_sale_attributes?: boolean;
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
      status?: string;
      source?: string;
      recommend_category_review?: boolean;
      category_review_reason?: string;
      primary_attribute_id?: number;
      secondary_attribute_id?: number;
      skc_attributes?: Array<{
        scope?: string;
        name?: string;
        value?: string;
        attribute_id?: number;
        attribute_value_id?: number;
        matched_by?: string;
      }>;
      sku_attributes?: Array<{
        scope?: string;
        name?: string;
        value?: string;
        attribute_id?: number;
        attribute_value_id?: number;
        matched_by?: string;
      }>;
      selection_summary?: string[];
      review_notes?: string[];
    };
    skc_patches?: Array<{
      supplier_code?: string;
      skc_name?: string;
      sale_name?: string;
      main_image_url?: string;
      sale_attribute?: {
        scope?: string;
        name?: string;
        value?: string;
        attribute_id?: number;
        attribute_value_id?: number;
        matched_by?: string;
      };
      sku_patches?: Array<{
        supplier_sku?: string;
        attributes?: Record<string, string>;
        sale_attributes?: Array<{
          scope?: string;
          name?: string;
          value?: string;
          attribute_id?: number;
          attribute_value_id?: number;
          matched_by?: string;
        }>;
      }>;
    }>;
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
