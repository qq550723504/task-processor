import { headers } from "next/headers";

import { resolvePublicAppOrigin } from "@/lib/server/zitadel-auth";
import type {
  StyleGalleryItem,
  StyleGalleryResponse,
} from "@/lib/types/style-gallery";

type StudioGalleryPayload = {
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

type GalleryFetchResult = {
  items: StyleGalleryItem[];
  loadError?: string;
};

export async function buildStyleGallery(): Promise<StyleGalleryResponse> {
  const { items, loadError } = await listDatabaseStudioItems();

  return {
    generatedAt: new Date().toISOString(),
    total: items.length,
    items,
    loadError,
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

export function resolveStyleGalleryApiOrigin(headerStore: Pick<Headers, "get">) {
  const forwardedHost = headerStore.get("x-forwarded-host")?.trim();
  const host = forwardedHost || headerStore.get("host")?.trim();
  if (!host) {
    return resolvePublicAppOrigin();
  }
  const forwardedProto = headerStore.get("x-forwarded-proto")?.trim();
  const protocol = forwardedProto || (host.startsWith("localhost") ? "http" : "https");
  return `${protocol}://${host}`;
}

async function listDatabaseStudioItems(): Promise<GalleryFetchResult> {
  try {
    const requestHeaders = await headers();
    const cookie = requestHeaders.get("cookie");
    const url = `${resolveStyleGalleryApiOrigin(requestHeaders)}/api/listing-kits/studio/sessions/gallery?limit=240`;
    const response = await fetch(url, {
      headers: {
        Accept: "application/json",
        ...(cookie ? { cookie } : {}),
      },
      cache: "no-store",
    });
    if (!response.ok) {
      return {
        items: [],
        loadError: `图库加载失败：${response.status}`,
      };
    }

    const payload = (await response.json()) as StudioGalleryPayload;

    return {
      items: (payload.items ?? [])
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
        } satisfies StyleGalleryItem)),
    };
  } catch (error) {
    return {
      items: [],
      loadError: error instanceof Error ? error.message : String(error),
    };
  }
}
