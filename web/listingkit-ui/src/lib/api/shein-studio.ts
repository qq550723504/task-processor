import type {
  SheinStudioGeneratedDesign,
  SheinStudioGenerateRequest,
  SheinStudioGenerateResponse,
  SheinStudioProductImagePrompt,
} from "@/lib/types/shein-studio";
import { apiAsyncRequest } from "@/lib/api/client";

export async function generateSheinStudioDesigns(
  body: SheinStudioGenerateRequest,
  options?: {
    onJobStarted?: (jobId: string) => void;
  },
) {
  const payload = await apiAsyncRequest<{
    prompt: string;
    printable_width?: number;
    printable_height?: number;
    image_model?: string;
    transparent_background?: boolean;
    warnings?: string[];
    images?: Array<{
      id: string;
      image_url?: string;
      imageUrl?: string;
      prompt?: string;
      revised_prompt?: string;
      revisedPrompt?: string;
      image_model?: string;
      imageModel?: string;
      transparent_background?: boolean;
      transparentBackground?: boolean;
      variation_intensity?: SheinStudioGenerateRequest["variationIntensity"];
      variationIntensity?: SheinStudioGenerateRequest["variationIntensity"];
      role?: string;
      role_label?: string;
      roleLabel?: string;
    }>;
  }>("/studio/designs", {
    body: {
      prompt: body.prompt,
      count: body.count,
      variation_intensity: body.variationIntensity,
      printable_width: body.printableWidth,
      printable_height: body.printableHeight,
      product_reference_image_urls: body.productReferenceImageUrls,
      image_model:
        body.transparentBackground
          ? "gpt-image-2"
          : body.imageModel?.trim() || undefined,
      transparent_background: body.transparentBackground,
    },
    timeoutMs: 3600000,
    onJobStarted: options?.onJobStarted,
  });
  return {
    prompt: payload.prompt,
    printableWidth: payload.printable_width,
    printableHeight: payload.printable_height,
    imageModel: payload.image_model,
    transparentBackground: payload.transparent_background,
    warnings: payload.warnings ?? [],
    images: (payload.images ?? []).map((image) => ({
      id: image.id,
      imageUrl: image.imageUrl ?? image.image_url,
      prompt: image.prompt ?? payload.prompt ?? body.prompt,
      revisedPrompt: image.revisedPrompt ?? image.revised_prompt,
      imageModel: image.imageModel ?? image.image_model ?? payload.image_model ?? body.imageModel,
      transparentBackground:
        image.transparentBackground ??
        image.transparent_background ??
        payload.transparent_background ??
        body.transparentBackground ??
        false,
      variationIntensity:
        image.variationIntensity ?? image.variation_intensity ?? body.variationIntensity,
    })),
  } satisfies SheinStudioGenerateResponse;
}

export async function generateSheinStudioProductImages(body: {
  prompt: string;
  productName?: string;
  categoryPath?: string[];
  styleName?: string;
  sourceDesignUrl?: string;
  productReferenceImageUrls?: string[];
  customPrompt?: string;
  imagePrompts?: SheinStudioProductImagePrompt[];
  count?: number;
}) {
  const payload = await apiAsyncRequest<{
    images?: Array<{
      id: string;
      image_url?: string;
      imageUrl?: string;
      revised_prompt?: string;
      revisedPrompt?: string;
      role?: string;
      role_label?: string;
      roleLabel?: string;
    }>;
  }>("/studio/product-images", {
    body: {
      prompt: body.prompt,
      product_name: body.productName,
      category_path: body.categoryPath,
      style_name: body.styleName,
      source_design_url: body.sourceDesignUrl,
      product_reference_image_urls: body.productReferenceImageUrls,
      custom_prompt: body.customPrompt,
      image_prompts: body.imagePrompts?.map((item) => ({
        role: item.role,
        prompt: item.prompt,
      })),
      count: body.count,
    },
    timeoutMs: 3600000,
  });
  return {
    images: (payload.images ?? []).map((image) => ({
      id: image.id,
      imageUrl: image.imageUrl ?? image.image_url,
      revisedPrompt: image.revisedPrompt ?? image.revised_prompt,
      role: image.role,
      roleLabel: image.roleLabel ?? image.role_label,
    })) satisfies SheinStudioGeneratedDesign[],
  };
}
