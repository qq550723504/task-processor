import { readdir, stat } from "node:fs/promises";
import path from "node:path";

import {
  buildListingKitProxyUrl,
  getListingKitUpstreamBase,
} from "@/app/api/listing-kits/proxy-url";
import { readSheinStudioStorage } from "@/lib/server/shein-studio-storage";
import type {
  SheinStyleGalleryItem,
  SheinStyleGalleryResponse,
  SheinStyleGallerySource,
} from "@/lib/types/shein-style-gallery";

const IMAGE_EXTENSIONS = new Set([".png", ".jpg", ".jpeg", ".webp"]);

export function getGalleryImageRoots() {
  const workspaceRoot = path.resolve(process.cwd(), "..", "..");
  return {
    legacy: path.join(process.cwd(), ".data", "shein-studio-assets"),
    published: path.join(
      workspaceRoot,
      "tmp",
      "productimage-published",
      "listingkit-inputs",
    ),
  };
}

export async function buildSheinStyleGallery(): Promise<SheinStyleGalleryResponse> {
  const [databaseItems, storedItems, legacyItems, publishedItems] = await Promise.all([
    listDatabaseStudioItems(),
    listStoredStudioItems(),
    listLegacyStudioItems(),
    listPublishedInputItems(),
  ]);

  const items = dedupeItems([...databaseItems, ...storedItems, ...legacyItems, ...publishedItems])
    .filter((item) => isAIGeneratedGallerySource(item.source))
    .sort(compareGalleryItems)
    .slice(0, 240);

  return {
    generatedAt: new Date().toISOString(),
    total: items.length,
    items,
    summary: {
      studioSaved: databaseItems.length,
      studioLegacy: legacyItems.length,
      publishedInputs: publishedItems.length,
      taskLinked: 0,
    },
  };
}

export function isAIGeneratedGallerySource(source: SheinStyleGallerySource) {
  return source === "studio_saved" || source === "studio_legacy" || source === "published_input";
}

export function resolveGalleryImagePath(source: string, segments: string[]) {
  const roots = getGalleryImageRoots();
  const root = source === "legacy" ? roots.legacy : source === "published" ? roots.published : "";
  if (!root || segments.length === 0) {
    return null;
  }

  const target = path.resolve(root, ...segments);
  const normalizedRoot = path.resolve(root);
  if (target !== normalizedRoot && !target.startsWith(`${normalizedRoot}${path.sep}`)) {
    return null;
  }

  return target;
}

async function listLegacyStudioItems(): Promise<SheinStyleGalleryItem[]> {
  const root = getGalleryImageRoots().legacy;
  const files = await safeListFiles(root);

  return files.map((file) => ({
    id: `legacy:${file.name}`,
    title: "Generated style",
    imageUrl: `/api/shein-studio/gallery/image/legacy/${encodeURIComponent(file.name)}`,
    source: "studio_legacy",
    sourceLabel: "Studio legacy",
    fileName: file.name,
    createdAt: file.mtime,
    updatedAt: file.mtime,
  }));
}

async function listStoredStudioItems(): Promise<SheinStyleGalleryItem[]> {
  const storage = await readSheinStudioStorage();
  const groups = [
    storage.draft
      ? {
          id: "draft",
          prompt: storage.draft.prompt,
          productName: storage.draft.selection?.productName,
          updatedAt: storage.draft.updatedAt,
          designs: storage.draft.designs,
        }
      : null,
    ...storage.batches.map((batch) => ({
      id: batch.id,
      prompt: batch.prompt,
      productName: batch.selection?.productName,
      updatedAt: batch.updatedAt,
      designs: batch.designs,
    })),
  ].filter((item): item is NonNullable<typeof item> => Boolean(item));

  return groups.flatMap((group) =>
    group.designs.flatMap((design, index) => {
      const imageUrl = design.imageUrl?.trim() || design.dataUrl?.trim();
      if (!imageUrl) {
        return [];
      }
      return [
        {
          id: `stored:${group.id}:${design.id}`,
          title: `AI style ${index + 1}`,
          imageUrl,
          source: "studio_saved",
          sourceLabel: "AI style",
          originalUrl: imageUrl,
          fileName: design.id,
          prompt: group.prompt,
          productName: group.productName,
          createdAt: group.updatedAt,
          updatedAt: group.updatedAt,
        } satisfies SheinStyleGalleryItem,
      ];
    }),
  );
}

async function listDatabaseStudioItems(): Promise<SheinStyleGalleryItem[]> {
  try {
    const url = buildListingKitProxyUrl(getListingKitUpstreamBase(), ["studio", "sessions", "gallery"], "limit=240");
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
        design_id?: string;
        image_url?: string;
        prompt?: string;
        product_name?: string;
        status?: string;
        created_at?: string;
        updated_at?: string;
      }>;
    };

    return (payload.items ?? [])
      .filter((item) => Boolean(item.design_id && item.image_url))
      .map((item, index) => ({
        id: `db:${item.session_id}:${item.design_id}`,
        title: `AI style ${index + 1}`,
        imageUrl: item.image_url!,
        source: "studio_saved",
        sourceLabel: "AI style",
        originalUrl: item.image_url,
        fileName: item.design_id,
        prompt: item.prompt,
        productName: item.product_name,
        taskStatus: item.status,
        createdAt: item.created_at,
        updatedAt: item.updated_at,
      } satisfies SheinStyleGalleryItem));
  } catch {
    return [];
  }
}

async function listPublishedInputItems(): Promise<SheinStyleGalleryItem[]> {
  const root = getGalleryImageRoots().published;
  const files = await safeListFiles(root, 2);

  return files.map((file) => ({
    id: `published:${file.relativePath}`,
    title: "Published input",
    imageUrl: `/api/shein-studio/gallery/image/published/${file.relativePath
      .split(/[\\/]/)
      .map(encodeURIComponent)
      .join("/")}`,
    source: "published_input",
    sourceLabel: "Published input",
    fileName: file.name,
    createdAt: file.mtime,
    updatedAt: file.mtime,
  }));
}

async function safeListFiles(root: string, depth = 1) {
  try {
    return await listImageFiles(root, root, depth);
  } catch {
    return [];
  }
}

async function listImageFiles(root: string, current: string, depth: number): Promise<
  Array<{ name: string; relativePath: string; mtime: string }>
> {
  const entries = await readdir(current, { withFileTypes: true });
  const files = await Promise.all(
    entries.map(async (entry) => {
      const fullPath = path.join(current, entry.name);
      if (entry.isDirectory() && depth > 0) {
        return listImageFiles(root, fullPath, depth - 1);
      }
      if (!entry.isFile() || !IMAGE_EXTENSIONS.has(path.extname(entry.name).toLowerCase())) {
        return [];
      }
      const info = await stat(fullPath);
      return [
        {
          name: entry.name,
          relativePath: path.relative(root, fullPath),
          mtime: info.mtime.toISOString(),
        },
      ];
    }),
  );

  return files.flat();
}

function dedupeItems(items: SheinStyleGalleryItem[]) {
  const seen = new Set<string>();
  return items.filter((item) => {
    const key = item.imageUrl;
    if (seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}

function compareGalleryItems(a: SheinStyleGalleryItem, b: SheinStyleGalleryItem) {
  const left = Date.parse(a.updatedAt ?? a.createdAt ?? "");
  const right = Date.parse(b.updatedAt ?? b.createdAt ?? "");
  return (Number.isNaN(right) ? 0 : right) - (Number.isNaN(left) ? 0 : left);
}
