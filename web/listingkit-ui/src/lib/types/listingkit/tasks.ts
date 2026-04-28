import type {
  ListingKitPreviewHeader,
  PreviewSlot,
  QueueItem,
  QueueSummary,
} from "./preview";
import type { PlatformPreviewPayload, SheinPreviewPayload } from "./shein";
import type { AssetGenerationOverview } from "./review";

export type ListingKitPreview = {
  task_id: string;
  status: string;
  selected_platform?: string;
  platforms?: string[];
  needs_review?: boolean;
  overview?: ListingKitPreviewHeader;
  asset_generation_overview?: AssetGenerationOverview;
  asset_generation_queue?: {
    summary?: QueueSummary;
    items?: QueueItem[];
  };
  asset_generation_tasks?: unknown[];
  asset_render_previews?: PreviewSlot[];
  platform_asset_render_previews?: unknown[];
  amazon?: PlatformPreviewPayload;
  shein?: SheinPreviewPayload;
  temu?: PlatformPreviewPayload;
  walmart?: PlatformPreviewPayload;
};

export type ListingKitChildTask = {
  kind?: string;
  task_id?: string;
  status?: string;
  error?: string;
};

export type SDSSyncSummary = {
  variant_id?: number;
  product_id?: number;
  prototype_group_id?: number;
  layer_id?: string;
  material_id?: number;
  mockup_image_urls?: string[];
  status?: string;
  error?: string;
};

export type ListingKitTaskResultData = {
  task_id?: string;
  status?: string;
  review_reasons?: string[];
  platforms?: string[];
  country?: string;
  language?: string;
  summary?: {
    source_type?: string;
    image_count?: number;
    variant_count?: number;
    needs_review?: boolean;
    warnings?: string[];
  };
  sds_sync?: SDSSyncSummary;
  child_tasks?: ListingKitChildTask[];
  created_at?: string;
  updated_at?: string;
};

export type ListingKitTaskResult = {
  task_id?: string;
  status?: string;
  result?: ListingKitTaskResultData;
  error?: string;
  review_reasons?: string[];
  created_at?: string;
  completed_at?: string;
};

export type ListingKitTaskListQuery = {
  status?: string;
  platform?: string;
  shein_workflow_status?: string;
  page?: number;
  page_size?: number;
};

export type ListingKitTaskListItem = {
  task_id: string;
  status?: string;
  platforms?: string[];
  title?: string;
  image_count?: number;
  product_name?: string;
  variant_label?: string;
  sds_sync_status?: string;
  shein_workflow_status?: string;
  shein_latest_submission_status?: string;
  shein_latest_submission_error?: string;
  error?: string;
  created_at?: string;
  updated_at?: string;
  completed_at?: string;
};

export type ListingKitTaskListPage = {
  page: number;
  page_size: number;
  total: number;
  items?: ListingKitTaskListItem[];
};

export type CreateListingKitTaskRequest = {
  image_urls?: string[];
  text?: string;
  product_url?: string;
  platforms: string[];
  shein_store_id?: number;
  country?: string;
  language?: string;
  options?: {
    image_strategy?: "ai_generated" | "sds_official" | "hybrid";
    process_images?: boolean;
    scene?: {
      scene_category?: string;
      scene_style?: string;
      background_tone?: string;
      composition?: string;
      props_level?: string;
      audience_hint?: string;
      custom_scene_hint?: string;
    };
    shein_studio?: {
      style_id?: string;
      style_name?: string;
      source_design_urls?: string[];
      product_image_urls?: string[];
      variant_product_images?: Array<{
        variant_sku?: string;
        color?: string;
        image_urls?: string[];
      }>;
      size_reference_image_urls?: string[];
      render_size_images_with_sds?: boolean;
    };
    sds?: {
      variant_id: number;
      parent_product_id?: number;
      prototype_group_id?: number;
      layer_id?: string;
      design_type?: string;
      fit_level?: number;
      resize_mode?: number;
      product_name?: string;
      product_sku?: string;
      product_english_name?: string;
      category_path?: string[];
      material?: string;
      material_description?: string;
      production_process?: string;
      product_performance?: string;
      applicable_scenarios?: string;
      washing_instructions?: string;
      special_description?: string;
      design_area?: string;
      picture_request?: string;
      is_electricity?: number;
      variant_sku?: string;
      variant_size?: string;
      variant_color?: string;
      variant_price?: number;
      variant_weight?: number;
      production_cycle?: number;
      blank_design_url?: string;
      template_image_url?: string;
      mask_image_url?: string;
      mockup_image_urls?: string[];
      style_id?: string;
      style_name?: string;
      variants?: Array<{
        variant_id: number;
        variant_sku?: string;
        size?: string;
        color?: string;
        price?: number;
        weight?: number;
        box_length?: number;
        box_width?: number;
        box_height?: number;
        production_cycle?: number;
        prototype_group_id?: number;
        layer_id?: string;
        template_image_url?: string;
        mask_image_url?: string;
        blank_design_url?: string;
        mockup_image_url?: string;
        mockup_image_urls?: string[];
      }>;
    };
  };
};

export type CreateListingKitTaskResponse = {
  task_id: string;
  status?: string;
  created_at?: string;
};

export type UploadImagesResponse = {
  image_urls?: string[];
};
