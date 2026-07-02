export type SheinPODImageLookupRecord = {
  task_id: string;
  store_id?: number;
  status?: string;
  prompt?: string;
  product_name?: string;
  supplier_code?: string;
  seller_sku?: string;
  shein_spu_name?: string;
  shein_version?: string;
  ai_original_image_url?: string;
  ai_original_image_key?: string;
  sds_main_image_url?: string;
  sds_gallery_image_urls?: string[];
  created_at?: string;
  updated_at?: string;
};

export type SheinPODImageLookupQuery = {
  query: string;
  limit?: number;
};

export type SheinPODImageLookupResponse = {
  items: SheinPODImageLookupRecord[];
  total: number;
};
