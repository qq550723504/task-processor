import { toImageProxyUrl } from "@/lib/utils/image-proxy-url";

type ImgproxyResizeMode = "fill" | "fit";

type ImgproxyThumbnailOptions = {
  width: number;
  height: number;
  mode?: ImgproxyResizeMode;
  format?: "avif" | "jpeg" | "png" | "webp";
};

const DEFAULT_IMGPROXY_FORMAT = "webp";
const DEFAULT_IMGPROXY_MODE: ImgproxyResizeMode = "fit";
const OSS_HOST = "oss.shuomiai.com";
const LEGACY_BUCKET = "listingkit-assets";
const COS_BUCKET = "shuomi-1303159911";

function configuredImgproxyBaseUrl() {
  return process.env.NEXT_PUBLIC_LISTINGKIT_IMGPROXY_BASE_URL?.trim().replace(/\/+$/, "") ?? "";
}

function parseOssObjectLocation(url: string) {
  try {
    const parsed = new URL(url);
    if (parsed.hostname.toLowerCase() !== OSS_HOST) {
      return null;
    }
    const segments = parsed.pathname.split("/").filter(Boolean);
    if (segments.length < 2) {
      return null;
    }
    if (segments[0] === LEGACY_BUCKET) {
      const [, ...keyParts] = segments;
      return {
        bucket: LEGACY_BUCKET,
        key: keyParts.join("/"),
      };
    }
    return {
      bucket: COS_BUCKET,
      key: segments.join("/"),
    };
  } catch {
    return null;
  }
}

export function toImgproxyThumbnailUrl(
  url?: string | null,
  options?: ImgproxyThumbnailOptions,
) {
  const trimmed = typeof url === "string" ? url.trim() : "";
  const imgproxyBase = configuredImgproxyBaseUrl();
  if (!trimmed || !imgproxyBase || !options) {
    return trimmed;
  }

  const objectLocation = parseOssObjectLocation(trimmed);
  if (!objectLocation) {
    return trimmed;
  }

  const mode = options.mode ?? DEFAULT_IMGPROXY_MODE;
  const format = options.format ?? DEFAULT_IMGPROXY_FORMAT;
  const width = Math.max(1, Math.floor(options.width));
  const height = Math.max(1, Math.floor(options.height));
  const source = `s3://${objectLocation.bucket}/${objectLocation.key}`;
  return `${imgproxyBase}/insecure/rs:${mode}:${width}:${height}/plain/${source}@${format}`;
}

export function toThumbnailPreviewUrl(
  url?: string | null,
  options?: ImgproxyThumbnailOptions,
) {
  const trimmed = typeof url === "string" ? url.trim() : "";
  if (!trimmed) {
    return "";
  }
  const imgproxyUrl = toImgproxyThumbnailUrl(trimmed, options);
  if (imgproxyUrl && imgproxyUrl !== trimmed) {
    return imgproxyUrl;
  }
  return toImageProxyUrl(trimmed);
}
