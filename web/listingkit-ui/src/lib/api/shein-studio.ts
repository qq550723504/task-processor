import type {
  SheinStudioGeneratedDesign,
  SheinStudioGenerateRequest,
  SheinStudioGenerateResponse,
  SheinStudioProductImagePrompt,
} from "@/lib/types/shein-studio";
import { apiRequest } from "@/lib/api/client";

export async function generateSheinStudioDesigns(
  body: SheinStudioGenerateRequest,
) {
  const payload = await apiRequest<{
    prompt: string;
    printable_width?: number;
    printable_height?: number;
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
  }>("/studio/designs", {
    method: "POST",
    body: {
      prompt: body.prompt,
      count: body.count,
      printable_width: body.printableWidth,
      printable_height: body.printableHeight,
      product_reference_image_urls: body.productReferenceImageUrls,
      image_model:
        body.transparentBackground || body.imageModel === "gpt-image-2"
          ? "gpt-image-2"
          : undefined,
      transparent_background: body.transparentBackground,
    },
    timeoutMs: 3600000,
  });
  return {
    prompt: payload.prompt,
    printableWidth: payload.printable_width,
    printableHeight: payload.printable_height,
    images: (payload.images ?? []).map((image) => ({
      id: image.id,
      imageUrl: image.imageUrl ?? image.image_url,
      revisedPrompt: image.revisedPrompt ?? image.revised_prompt,
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
  const payload = await apiRequest<{
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
    method: "POST",
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
