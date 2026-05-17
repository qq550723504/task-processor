import {
  buildListingKitProxyUrl,
  getListingKitUpstreamBase,
} from "@/app/api/listing-kits/proxy-url";
import type {
  StyleGalleryItem,
  StyleGalleryResponse,
} from "@/lib/types/style-gallery";

export async function buildStyleGallery(): Promise<StyleGalleryResponse> {
  const items = await listDatabaseStudioItems();

  return {
    generatedAt: new Date().toISOString(),
    total: items.length,
    items,
    summary: {
      studioSaved: items.length,
      studioLegacy: 0,
      publishedInputs: 0,
      taskLinked: 0,
    },
  };
}

export function normalizeStyleGalleryImageUrl(url: string) {
  try {
    const parsed = new URL(url);
    const prefix = "/api/v1/listing-kits/uploads/files/";
    if (!parsed.pathname.startsWith(prefix)) {
      return url;
    }
    return `${parsed.pathname.replace(prefix, "/api/listing-kits/uploads/files/")}${parsed.search}`;
  } catch {
    return url;
  }
}

async function listDatabaseStudioItems(): Promise<StyleGalleryItem[]> {
  try {
    const url = buildListingKitProxyUrl(
      getListingKitUpstreamBase(),
      ["studio", "sessions", "gallery"],
      "limit=240",
    );
    const response = await fetch(url, {
      headers: {
        Accept: "application/json",
      },
      cache: "no-store",
    });
    if (!response.ok) {
      return [];
    }

    const payload = (await response.json()) as {
      items?: Array<{
        session_id?: string;
        tenant_id?: string;
        design_id?: string;
        image_url?: string;
        prompt?: string;
        product_name?: string;
        status?: string;
        created_at?: string;
        updated_at?: string;
        revised_prompt?: string;
        image_model?: string;
        transparent_background?: boolean;
        variation_intensity?: string;
      }>;
    };

    return (payload.items ?? [])
      .filter((item) => Boolean(item.design_id && item.image_url))
      .map((item, index) => ({
        id: `db:${item.session_id}:${item.design_id}`,
        title: `AI style ${index + 1}`,
        imageUrl: normalizeStyleGalleryImageUrl(item.image_url!),
        source: "studio_saved",
        sourceLabel: "AI style",
        originalUrl: item.image_url,
        fileName: item.design_id,
        prompt: item.prompt,
        imageModel: item.image_model,
        transparentBackground: item.transparent_background,
        variationIntensity: item.variation_intensity,
        productName: item.product_name,
        taskStatus: item.status,
        createdAt: item.created_at,
        updatedAt: item.updated_at,
        variantLabel: item.tenant_id,
      } satisfies StyleGalleryItem));
  } catch {
    return [];
  }
}
