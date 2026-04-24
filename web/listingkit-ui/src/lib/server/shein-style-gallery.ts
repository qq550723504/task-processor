import { readdir, stat } from "node:fs/promises";
import path from "node:path";

import { getListingKitUpstreamBase } from "@/app/api/listing-kits/proxy-url";
import type {
  SheinStyleGalleryItem,
  SheinStyleGalleryResponse,
  SheinStyleGallerySource,
} from "@/lib/types/shein-style-gallery";

type TaskListResponse = {
  items?: TaskListItem[];
};

type TaskListItem = {
  task_id?: string;
  status?: string;
  title?: string;
  product_name?: string;
  variant_label?: string;
  created_at?: string;
  updated_at?: string;
};

type TaskDetail = {
  task_id?: string;
  status?: string;
  result?: unknown;
  created_at?: string;
  completed_at?: string;
};

const IMAGE_EXTENSIONS = new Set([".png", ".jpg", ".jpeg", ".webp"]);
const MAX_TASK_DETAILS = 30;

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
  const [legacyItems, publishedItems, taskItems] = await Promise.all([
    listLegacyStudioItems(),
    listPublishedInputItems(),
    listTaskLinkedItems(),
  ]);

  const items = dedupeItems([...taskItems, ...legacyItems, ...publishedItems])
    .sort(compareGalleryItems)
    .slice(0, 240);

  return {
    generatedAt: new Date().toISOString(),
    total: items.length,
    items,
    summary: {
      studioLegacy: legacyItems.length,
      publishedInputs: publishedItems.length,
      taskLinked: taskItems.length,
    },
  };
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

async function listTaskLinkedItems(): Promise<SheinStyleGalleryItem[]> {
  const tasks = await fetchTaskList();
  const details = await Promise.all(
    tasks.slice(0, MAX_TASK_DETAILS).map((task) => fetchTaskDetail(task.task_id ?? "")),
  );

  const out: SheinStyleGalleryItem[] = [];
  details.forEach((detail, index) => {
    if (!detail?.result) {
      return;
    }
    const task = tasks[index];
    const context = {
      taskId: detail.task_id ?? task.task_id,
      taskStatus: detail.status ?? task.status,
      prompt: extractString((detail.result as Record<string, unknown>).canonical_product, "description"),
      productName: task.product_name,
      variantLabel: task.variant_label,
      createdAt: detail.created_at ?? task.created_at,
      updatedAt: detail.completed_at ?? task.updated_at,
    };

    out.push(...extractTaskImages(detail.result, context));
  });
  return out;
}

function extractTaskImages(
  result: unknown,
  context: {
    taskId?: string;
    taskStatus?: string;
    prompt?: string;
    productName?: string;
    variantLabel?: string;
    createdAt?: string;
    updatedAt?: string;
  },
): SheinStyleGalleryItem[] {
  const root = asRecord(result);
  if (!root) {
    return [];
  }

  const items: SheinStyleGalleryItem[] = [];
  addUrlGroup(items, root, ["catalog_product", "images"], "task_source", "Source product", context);
  addUrlGroup(items, root, ["canonical_product", "images"], "task_source", "Source product", context);
  addUrlGroup(items, root, ["sds_sync", "mockup_image_urls"], "task_mockup", "SDS mockup", context);
  addUrlGroup(items, root, ["shein", "images", "main_image"], "task_shein", "SHEIN main", context);
  addUrlGroup(items, root, ["shein", "images", "gallery"], "task_shein", "SHEIN gallery", context);
  addUrlGroup(items, root, ["shein", "request_draft", "image_info", "main_image"], "task_shein", "SHEIN draft", context);
  addUrlGroup(items, root, ["shein", "request_draft", "image_info", "gallery"], "task_shein", "SHEIN draft", context);

  return items;
}

function addUrlGroup(
  items: SheinStyleGalleryItem[],
  root: Record<string, unknown>,
  pathSegments: string[],
  source: SheinStyleGallerySource,
  label: string,
  context: {
    taskId?: string;
    taskStatus?: string;
    prompt?: string;
    productName?: string;
    variantLabel?: string;
    createdAt?: string;
    updatedAt?: string;
  },
) {
  const urls = collectUrls(readPath(root, pathSegments));
  urls.forEach((url, index) => {
    const imageUrl = toDisplayImageUrl(url);
    const suffix = index > 0 ? ` ${index + 1}` : "";
    items.push({
      id: `${context.taskId ?? "task"}:${source}:${pathSegments.join(".")}:${url}`,
      title: `${label}${suffix}`,
      imageUrl,
      source,
      sourceLabel: label,
      originalUrl: url,
      fileName: fileNameFromUrl(url),
      ...context,
    });
  });
}

function toDisplayImageUrl(url: string) {
  const uploadMatch = url.match(/\/uploads\/files\/(\d{8})\/([^/?#]+)/);
  if (uploadMatch) {
    return `/api/shein-studio/gallery/image/published/${uploadMatch[1]}/${encodeURIComponent(uploadMatch[2])}`;
  }
  return url;
}

async function fetchTaskList(): Promise<TaskListItem[]> {
  try {
    const response = await fetch(`${getListingKitUpstreamBase()}/tasks?page=1&page_size=80`, {
      cache: "no-store",
    });
    if (!response.ok) {
      return [];
    }
    const payload = (await response.json()) as TaskListResponse;
    return payload.items ?? [];
  } catch {
    return [];
  }
}

async function fetchTaskDetail(taskId: string): Promise<TaskDetail | null> {
  if (!taskId) {
    return null;
  }

  try {
    const response = await fetch(`${getListingKitUpstreamBase()}/tasks/${encodeURIComponent(taskId)}`, {
      cache: "no-store",
    });
    if (!response.ok) {
      return null;
    }
    return (await response.json()) as TaskDetail;
  } catch {
    return null;
  }
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

function collectUrls(value: unknown): string[] {
  if (typeof value === "string") {
    return isImageUrl(value) ? [value] : [];
  }
  if (Array.isArray(value)) {
    return value.flatMap(collectUrls);
  }
  const record = asRecord(value);
  if (!record) {
    return [];
  }
  if (typeof record.url === "string") {
    return collectUrls(record.url);
  }
  if (typeof record.image_url === "string") {
    return collectUrls(record.image_url);
  }
  return [];
}

function readPath(root: Record<string, unknown>, segments: string[]) {
  let current: unknown = root;
  for (const segment of segments) {
    const record = asRecord(current);
    if (!record) {
      return undefined;
    }
    current = record[segment];
  }
  return current;
}

function extractString(value: unknown, key: string) {
  const record = asRecord(value);
  const item = record?.[key];
  return typeof item === "string" ? item : undefined;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function isImageUrl(value: string) {
  return /\.(png|jpe?g|webp)(\?|#|$)/i.test(value);
}

function fileNameFromUrl(value: string) {
  const clean = value.split("?")[0]?.split("#")[0] ?? value;
  return decodeURIComponent(clean.split("/").pop() ?? clean);
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
