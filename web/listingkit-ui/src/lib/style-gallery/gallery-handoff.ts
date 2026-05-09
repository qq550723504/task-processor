import type { StyleGalleryItem } from "@/lib/types/style-gallery";
import type { SheinStudioGeneratedDesign } from "@/lib/types/shein-studio";

export const STYLE_GALLERY_HANDOFF_STORAGE_KEY =
  "listingkit:style-gallery:handoff";

const HANDOFF_TTL_MS = 24 * 60 * 60 * 1000;
const WARNING_RATIO_DELTA = 0.15;
const BLOCKING_RATIO_DELTA = 0.25;

export type StyleGalleryHandoff = {
  id: string;
  title: string;
  imageUrl: string;
  source: string;
  createdAt: string;
  prompt?: string;
  productName?: string;
  width?: number;
  height?: number;
};

export type SDSRatioMatchStatus = "unknown" | "pass" | "warning" | "blocking";

export type SDSRatioMatch = {
  status: SDSRatioMatchStatus;
  sourceRatio?: number;
  targetRatio?: number;
  relativeDifference?: number;
  message: string;
};

type RatioInput = {
  sourceWidth?: number;
  sourceHeight?: number;
  targetWidth?: number;
  targetHeight?: number;
};

function isPositiveNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value) && value > 0;
}

function isHandoffPayload(value: unknown): value is StyleGalleryHandoff {
  if (!value || typeof value !== "object") {
    return false;
  }
  const payload = value as StyleGalleryHandoff;
  return (
    typeof payload.id === "string" &&
    typeof payload.title === "string" &&
    typeof payload.imageUrl === "string" &&
    typeof payload.source === "string" &&
    typeof payload.createdAt === "string"
  );
}

function safeStorage() {
  if (typeof window === "undefined") {
    return null;
  }
  return window.localStorage;
}

export function buildStyleGalleryHandoff(
  item: StyleGalleryItem,
  dimensions?: { width?: number; height?: number },
): StyleGalleryHandoff {
  return {
    id: item.id,
    title: item.title,
    imageUrl: item.imageUrl,
    source: item.source,
    prompt: item.prompt,
    productName: item.productName,
    width: dimensions?.width,
    height: dimensions?.height,
    createdAt: new Date().toISOString(),
  };
}

export function saveStyleGalleryHandoff(
  handoff: StyleGalleryHandoff,
  storage: Pick<Storage, "setItem"> | null = safeStorage(),
) {
  storage?.setItem(STYLE_GALLERY_HANDOFF_STORAGE_KEY, JSON.stringify(handoff));
}

export function consumeStyleGalleryHandoff(
  storage: Pick<Storage, "getItem" | "removeItem"> | null = safeStorage(),
  now: Date = new Date(),
) {
  const raw = storage?.getItem(STYLE_GALLERY_HANDOFF_STORAGE_KEY);
  if (!raw) {
    return null;
  }
  storage?.removeItem(STYLE_GALLERY_HANDOFF_STORAGE_KEY);

  try {
    const payload = JSON.parse(raw) as unknown;
    if (!isHandoffPayload(payload)) {
      return null;
    }
    const createdAt = new Date(payload.createdAt).getTime();
    if (!Number.isFinite(createdAt) || now.getTime() - createdAt > HANDOFF_TTL_MS) {
      return null;
    }
    return payload;
  } catch {
    return null;
  }
}

export function styleGalleryHandoffToDesign(
  handoff: StyleGalleryHandoff,
): SheinStudioGeneratedDesign {
  return {
    id: `gallery-${handoff.id}`,
    imageUrl: handoff.imageUrl,
    revisedPrompt: handoff.prompt || handoff.productName || handoff.title,
    role: "gallery",
    roleLabel: "图库导入",
    sourceWidth: handoff.width,
    sourceHeight: handoff.height,
  };
}

export function evaluateSDSRatioMatch(input: RatioInput): SDSRatioMatch {
  if (
    !isPositiveNumber(input.sourceWidth) ||
    !isPositiveNumber(input.sourceHeight) ||
    !isPositiveNumber(input.targetWidth) ||
    !isPositiveNumber(input.targetHeight)
  ) {
    return {
      status: "unknown",
      message: "缺少图库图片或 SDS 印刷区域尺寸，无法自动判断比例是否匹配。",
    };
  }

  const sourceRatio = input.sourceWidth / input.sourceHeight;
  const targetRatio = input.targetWidth / input.targetHeight;
  const relativeDifference = Math.abs(sourceRatio - targetRatio) / targetRatio;

  if (relativeDifference > BLOCKING_RATIO_DELTA) {
    return {
      status: "blocking",
      sourceRatio,
      targetRatio,
      relativeDifference,
      message: "图库图片比例与 SDS 款式比例差异过大，请换图或更换 SDS 款式。",
    };
  }
  if (relativeDifference > WARNING_RATIO_DELTA) {
    return {
      status: "warning",
      sourceRatio,
      targetRatio,
      relativeDifference,
      message: "图库图片比例与 SDS 款式比例有差异，继续生成可能产生裁切或留白。",
    };
  }
  return {
    status: "pass",
    sourceRatio,
    targetRatio,
    relativeDifference,
    message: "图库图片比例与 SDS 款式比例接近。",
  };
}
