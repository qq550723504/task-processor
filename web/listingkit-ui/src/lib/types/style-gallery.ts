export type StyleGallerySource =
  | "studio_saved"
  | "studio_legacy"
  | "published_input"
  | "task_source"
  | "task_mockup"
  | "task_shein";

export type StyleGalleryItem = {
  id: string;
  title: string;
  imageUrl: string;
  source: StyleGallerySource;
  sourceLabel: string;
  originalUrl?: string;
  fileName?: string;
  taskId?: string;
  taskStatus?: string;
  prompt?: string;
  imageModel?: string;
  transparentBackground?: boolean;
  variationIntensity?: string;
  productName?: string;
  variantLabel?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type StyleGalleryResponse = {
  generatedAt: string;
  total: number;
  items: StyleGalleryItem[];
  loadError?: string;
  summary: {
    studioSaved: number;
    studioLegacy: number;
    publishedInputs: number;
    taskLinked: number;
  };
};
