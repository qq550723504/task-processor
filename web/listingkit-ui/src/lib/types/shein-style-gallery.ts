export type SheinStyleGallerySource =
  | "studio_legacy"
  | "published_input"
  | "task_source"
  | "task_mockup"
  | "task_shein";

export type SheinStyleGalleryItem = {
  id: string;
  title: string;
  imageUrl: string;
  source: SheinStyleGallerySource;
  sourceLabel: string;
  originalUrl?: string;
  fileName?: string;
  taskId?: string;
  taskStatus?: string;
  prompt?: string;
  productName?: string;
  variantLabel?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type SheinStyleGalleryResponse = {
  generatedAt: string;
  total: number;
  items: SheinStyleGalleryItem[];
  summary: {
    studioLegacy: number;
    publishedInputs: number;
    taskLinked: number;
  };
};
