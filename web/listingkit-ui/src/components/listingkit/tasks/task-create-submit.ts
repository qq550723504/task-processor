import {
  buildSDSOptions,
  buildSceneOptions,
  parseImageUrls,
  parseOptionalPositiveInt,
  type FormValues,
} from "@/components/listingkit/tasks/task-create-form-model";
import type { TaskCreateDraft } from "@/components/listingkit/tasks/task-create-draft";
import { loadSDSListingKitMetadata } from "@/lib/sds/product-metadata";

export async function buildTaskCreateSubmission({
  selectedVariantIds,
  values,
}: {
  selectedVariantIds?: number[];
  values: FormValues;
}) {
  const text = (values.text ?? "").trim();
  const imageUrls = values.imageUrls ?? "";
  const productUrl = (values.productUrl ?? "").trim();
  const parsedImageUrls = parseImageUrls(imageUrls);

  if (!text && parsedImageUrls.length === 0 && !productUrl) {
    return {
      ok: false as const,
      message: "请至少提供商品标题、图片链接或商品链接中的一种。",
    };
  }

  const draft = buildTaskCreateDraft({
    imageUrls,
    productUrl,
    text,
    values,
  });
  const sceneOptions = buildSceneOptions(values);
  const sdsOptions = buildSDSOptions(values);
  if (values.sdsEnabled && !sdsOptions) {
    return {
      ok: false as const,
      message: "启用 SDS 同步时，必须填写有效的 Variant ID。",
    };
  }

  const sdsMetadata =
    sdsOptions && sdsOptions.variant_id && sdsOptions.parent_product_id
      ? await loadSDSListingKitMetadata({
          parentProductId: sdsOptions.parent_product_id,
          variantId: sdsOptions.variant_id,
          selectedVariantIds,
        })
      : {};
  const enrichedSDSOptions = sdsOptions
    ? {
        ...sdsMetadata,
        ...sdsOptions,
      }
    : undefined;
  const sheinStoreId = parseOptionalPositiveInt(values.sheinStoreId ?? "");
  const options = {
    ...(sceneOptions ? { process_images: true } : {}),
    ...(enrichedSDSOptions && !sceneOptions ? { process_images: false } : {}),
    ...(sceneOptions ? { scene: sceneOptions } : {}),
    ...(enrichedSDSOptions ? { sds: enrichedSDSOptions } : {}),
  };

  return {
    ok: true as const,
    draft,
    request: {
      text: draft.text,
      image_urls: parsedImageUrls,
      platforms: values.platforms,
      ...(sheinStoreId ? { shein_store_id: sheinStoreId } : {}),
      ...(draft.productUrl ? { product_url: draft.productUrl } : {}),
      ...(Object.keys(options).length > 0 ? { options } : {}),
    },
  };
}

function buildTaskCreateDraft({
  imageUrls,
  productUrl,
  text,
  values,
}: {
  imageUrls: string;
  productUrl: string;
  text: string;
  values: FormValues;
}) {
  return {
    text,
    imageUrls,
    productUrl,
    platforms: values.platforms,
    sheinStoreId: values.sheinStoreId,
    sdsEnabled: values.sdsEnabled,
    sdsVariantId: values.sdsVariantId,
    sdsParentProductId: values.sdsParentProductId,
    sdsPrototypeGroupId: values.sdsPrototypeGroupId,
    sdsLayerId: values.sdsLayerId,
    sdsDesignType: values.sdsDesignType,
    sdsFitLevel: values.sdsFitLevel,
    sdsResizeMode: values.sdsResizeMode,
    sceneCategory: values.sceneCategory,
    sceneStyle: values.sceneStyle,
    backgroundTone: values.backgroundTone,
    composition: values.composition,
    propsLevel: values.propsLevel,
    audienceHint: values.audienceHint,
    customSceneHint: values.customSceneHint,
  } satisfies TaskCreateDraft;
}
