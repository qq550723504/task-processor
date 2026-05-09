import type {
  SDSSyncSummary,
  SheinImageInfo,
  SheinPreviewPayload,
} from "@/lib/types/listingkit";

export type SheinPreviewImage = {
  id: string;
  label: string;
  url: string;
};

export type SheinPreviewImageGroups = {
  productImages: SheinPreviewImage[];
  mockupImages: SheinPreviewImage[];
};

function firstUrl(values: Array<string | null | undefined>) {
  return values.find((value) => typeof value === "string" && value.trim())?.trim();
}

function pushImage(
  images: SheinPreviewImage[],
  seen: Set<string>,
  label: string,
  url?: string | null,
) {
  const trimmed = typeof url === "string" ? url.trim() : "";
  if (!trimmed || seen.has(trimmed)) {
    return;
  }
  seen.add(trimmed);
  images.push({
    id: `${label}:${images.length}`,
    label,
    url: trimmed,
  });
}

function collectImageInfoUrls(
  images: SheinPreviewImage[],
  seen: Set<string>,
  label: string,
  info?: SheinImageInfo | null,
) {
  if (!info) {
    return;
  }

  pushImage(images, seen, `${label} main`, info.main_image);
  pushImage(images, seen, `${label} white bg`, info.white_bg);
  info.gallery?.forEach((url, index) => {
    pushImage(images, seen, `${label} gallery ${index + 1}`, url);
  });
  info.image_info_list?.forEach((image, index) => {
    pushImage(
      images,
      seen,
      `${label} image ${index + 1}`,
      image.image_url ?? image.imageUrl,
    );
  });
}

export function collectSheinPreviewImages(
  shein?: SheinPreviewPayload | null,
  sdsSync?: SDSSyncSummary | null,
): SheinPreviewImage[] {
  const groups = collectSheinPreviewImageGroups(shein, sdsSync);
  if (groups.productImages.length > 0) {
    return groups.productImages;
  }
  return groups.mockupImages;
}

export function collectSheinPreviewImageGroups(
  shein?: SheinPreviewPayload | null,
  sdsSync?: SDSSyncSummary | null,
): SheinPreviewImageGroups {
  if (!shein && !sdsSync) {
    return { productImages: [], mockupImages: [] };
  }

  const productImages: SheinPreviewImage[] = [];
  const productSeen = new Set<string>();
  const mockupImages: SheinPreviewImage[] = [];
  const mockupSeen = new Set<string>();
  const requestDraft = shein?.request_draft;
  const previewProduct = shein?.preview_product;
  let hasFinalReviewImages = false;

  shein?.final_review?.images?.forEach((image, index) => {
    const label =
      image.main || image.role === "main"
        ? "Final review main"
        : `Final review image ${index + 1}`;
    pushImage(productImages, productSeen, label, image.url);
  });
  hasFinalReviewImages = productImages.length > 0;

  const appendSourceImages = () => {
    const appendSourcesFromImageInfo = (label: string, info?: SheinImageInfo | null) => {
      info?.source?.forEach((url, index) => {
        pushImage(productImages, productSeen, `${label} source ${index + 1}`, url);
      });
    };

    shein?.source_product?.image_urls?.forEach((url, index) => {
      pushImage(productImages, productSeen, `Source product ${index + 1}`, url);
    });
    appendSourcesFromImageInfo("Preview product", previewProduct?.image_info);
    appendSourcesFromImageInfo("Request draft", requestDraft?.image_info);
  };

  if (!hasFinalReviewImages) {
    collectImageInfoUrls(productImages, productSeen, "Preview product", previewProduct?.image_info);
    previewProduct?.skc_list?.forEach((skc, skcIndex) => {
      collectImageInfoUrls(productImages, productSeen, `Preview SKC ${skcIndex + 1}`, skc.image_info);
      skc.sku_list?.forEach((sku, skuIndex) => {
        collectImageInfoUrls(
          productImages,
          productSeen,
          `Preview SKU ${skcIndex + 1}.${skuIndex + 1}`,
          sku.image_info,
        );
      });
    });
  }
  sdsSync?.mockup_image_urls?.forEach((url, index) => {
    const trimmed = typeof url === "string" ? url.trim() : "";
    if (!trimmed || productSeen.has(trimmed)) {
      return;
    }
    pushImage(mockupImages, mockupSeen, `SDS mockup ${index + 1}`, trimmed);
  });
  if (!hasFinalReviewImages) {
    collectImageInfoUrls(productImages, productSeen, "Request draft", requestDraft?.image_info);
    requestDraft?.skc_list?.forEach((skc, skcIndex) => {
      collectImageInfoUrls(productImages, productSeen, `Draft SKC ${skcIndex + 1}`, skc.image_info);
      skc.sku_list?.forEach((sku, skuIndex) => {
        pushImage(
          productImages,
          productSeen,
          `Draft SKU ${skcIndex + 1}.${skuIndex + 1} main`,
          sku.main_image,
        );
        collectImageInfoUrls(
          productImages,
          productSeen,
          `Draft SKU ${skcIndex + 1}.${skuIndex + 1}`,
          sku.image_info,
        );
      });
    });
  }
  if (productImages.length === 0) {
    appendSourceImages();
  }

  return { productImages, mockupImages };
}

export function firstSheinPreviewImageUrl(shein?: SheinPreviewPayload | null) {
  return firstUrl(collectSheinPreviewImages(shein).map((image) => image.url));
}
