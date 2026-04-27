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
  if (!shein && !sdsSync) {
    return [];
  }

  const images: SheinPreviewImage[] = [];
  const seen = new Set<string>();
  const requestDraft = shein?.request_draft;
  const previewProduct = shein?.preview_product;

  const appendSourceImages = () => {
    const appendSourcesFromImageInfo = (label: string, info?: SheinImageInfo | null) => {
      info?.source?.forEach((url, index) => {
        pushImage(images, seen, `${label} source ${index + 1}`, url);
      });
    };

    shein?.source_product?.image_urls?.forEach((url, index) => {
      pushImage(images, seen, `Source product ${index + 1}`, url);
    });
    appendSourcesFromImageInfo("Preview product", previewProduct?.image_info);
    appendSourcesFromImageInfo("Request draft", requestDraft?.image_info);
  };

  collectImageInfoUrls(images, seen, "Preview product", previewProduct?.image_info);
  previewProduct?.skc_list?.forEach((skc, skcIndex) => {
    collectImageInfoUrls(images, seen, `Preview SKC ${skcIndex + 1}`, skc.image_info);
    skc.sku_list?.forEach((sku, skuIndex) => {
      collectImageInfoUrls(
        images,
        seen,
        `Preview SKU ${skcIndex + 1}.${skuIndex + 1}`,
        sku.image_info,
      );
    });
  });
  sdsSync?.mockup_image_urls?.forEach((url, index) => {
    pushImage(images, seen, `SDS mockup ${index + 1}`, url);
  });
  collectImageInfoUrls(images, seen, "Request draft", requestDraft?.image_info);
  requestDraft?.skc_list?.forEach((skc, skcIndex) => {
    collectImageInfoUrls(images, seen, `Draft SKC ${skcIndex + 1}`, skc.image_info);
    skc.sku_list?.forEach((sku, skuIndex) => {
      pushImage(
        images,
        seen,
        `Draft SKU ${skcIndex + 1}.${skuIndex + 1} main`,
        sku.main_image,
      );
      collectImageInfoUrls(
        images,
        seen,
        `Draft SKU ${skcIndex + 1}.${skuIndex + 1}`,
        sku.image_info,
      );
    });
  });
  if (images.length === 0) {
    appendSourceImages();
  }

  return images;
}

export function firstSheinPreviewImageUrl(shein?: SheinPreviewPayload | null) {
  return firstUrl(collectSheinPreviewImages(shein).map((image) => image.url));
}
