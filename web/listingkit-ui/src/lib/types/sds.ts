export type SDSPrototypeLayer = {
  id: string;
  name?: string;
  isMasterMap?: number;
  printWidth?: number;
  printHeight?: number;
  maskUrl?: string;
  maskShowUrl?: string;
  maskThumbnailUrl?: string;
  thumbnailUrl?: string;
  imageUrl?: string;
};

export type SDSPrototypeResultGroup = {
  id?: string;
  resultImage?: string;
  sort?: number;
  faceSheetState?: boolean;
};

export type SDSDesignPrototype = {
  prototypeId?: string;
  prototypeGroupId?: number;
  prototypeLayerList?: SDSPrototypeLayer[];
  prototypeResultGroups?: SDSPrototypeResultGroup[];
};

export type SDSProductVariant = {
  id: number;
  parent_id?: number;
  sku?: string;
  size?: string;
  color_name?: string;
  currentPrice?: number;
  weight?: number;
  box_length?: number;
  box_width?: number;
  box_height?: number;
  productionCycle?: number;
  isElectricity?: number;
  on_sale_status?: number;
  hotSellStatus?: number;
  issuingBayArea?: {
    name?: string;
    countryCode?: string;
  };
  img_url?: string;
  designPrototype?: SDSDesignPrototype;
};

export type SDSProductDetails = {
  production_process?: string;
  material_description?: string;
  product_performance?: string;
  applicable_scenarios?: string;
  washing_instructions?: string;
  special_description?: string;
  design_area?: string;
  picture_request?: string;
  product_size?: string;
  packaging_specification?: string;
};

export type SDSTexture = {
  name?: string;
};

export type SDSProductVariantSelection = {
  productId: number;
  parentProductId: number;
  variantId: number;
  variants?: SDSSelectedProductVariant[];
  selectedVariantIds?: number[];
  prototypeGroupId: number;
  layerId: string;
  designType?: string;
  productSize?: string;
  packagingSpecification?: string;
  productName: string;
  variantLabel: string;
  printableWidth?: number;
  printableHeight?: number;
  templateImageUrl?: string;
  maskImageUrl?: string;
  blankDesignUrl?: string;
  mockupImageUrl?: string;
  mockupImageUrls?: string[];
  sizeReferenceImageUrls?: string[];
};

export type SDSSelectedProductVariant = {
  variantId: number;
  variantSku?: string;
  size?: string;
  color?: string;
  price?: number;
  weight?: number;
  boxLength?: number;
  boxWidth?: number;
  boxHeight?: number;
  productionCycle?: number;
  prototypeGroupId?: number;
  layerId?: string;
  templateImageUrl?: string;
  maskImageUrl?: string;
  blankDesignUrl?: string;
  mockupImageUrl?: string;
  mockupImageUrls?: string[];
  sizeReferenceImageUrls?: string[];
};

export type SDSSubproducts = {
  items?: SDSProductVariant[];
};

export type SDSProductSummary = {
  id: number;
  parent_id?: number;
  parentSku?: string;
  name: string;
  product_name?: string;
  productName?: string;
  product_name_multi?: string;
  sku?: string;
  english_name?: string;
  declaration_name?: string;
  declaration_english_name?: string;
  blankDesignUrl?: string;
  img_url?: string;
  min_price?: number;
  currentPrice?: number;
  weight?: number;
  weightMin?: number;
  weightMax?: number;
  minWeight?: number;
  productionCycle?: number;
  isElectricity?: number;
  on_sale_status?: number;
  hotSellStatus?: number;
  issuingBayArea?: {
    name?: string;
    countryCode?: string;
  };
  texture?: SDSTexture;
  product_details?: SDSProductDetails;
  categories?: Array<{ id: number; name: string }>;
  subproducts?: SDSSubproducts;
};

export type SDSProductListResponse = {
  totalCount?: number;
  page?: number;
  size?: number;
  items?: SDSProductSummary[];
};

export type SDSShipmentArea = {
  value: string;
  label: string;
  totalCount: number;
};

export type SDSCategory = {
  id: number;
  name: string;
  count: number;
};

export type SDSProductDetail = SDSProductSummary & {
  subproducts?: SDSSubproducts;
};
