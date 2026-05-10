import type { SheinPreviewImage } from "@/components/listingkit/shein/shein-preview-image";

export type ImageRole = "main" | "gallery" | "swatch" | "size_map" | "skc";

export function buildImageRoleOverrides(
  imageUrls: string[],
  roleByUrl: Record<string, ImageRole>,
  mainUrl?: string,
) {
  const overrides: Record<string, ImageRole> = {};
  for (const url of imageUrls) {
    const role = roleByUrl[url] ?? "gallery";
    overrides[url] = url === mainUrl && role === "gallery" ? "main" : role;
  }
  return overrides;
}

export function suggestImageRoles(
  images: SheinPreviewImage[],
  currentRoles: Record<string, ImageRole>,
  currentMainUrl?: string,
) {
  const roles: Record<string, ImageRole> = {};
  for (const image of images) {
    roles[image.url] = currentRoles[image.url] ?? "gallery";
  }
  const mainUrl =
    images.find((image) => image.url === currentMainUrl)?.url ?? images[0]?.url;
  if (mainUrl) {
    roles[mainUrl] = "main";
  }
  const sizeImage = images.find((image) =>
    isLikelySizeMapImage(image, roles[image.url]),
  );
  if (sizeImage) {
    roles[sizeImage.url] = "size_map";
  }
  const skcImage = images.find(
    (image) => image.url !== mainUrl && roles[image.url] === "skc",
  );
  const swatchSource =
    skcImage ??
    images.find(
      (image) => image.url !== mainUrl && roles[image.url] === "gallery",
    );
  if (swatchSource && roles[swatchSource.url] !== "size_map") {
    roles[swatchSource.url] = "swatch";
  }
  return { mainUrl, roles };
}

export function isLikelySizeMapImage(
  image: SheinPreviewImage,
  role?: ImageRole,
) {
  if (role === "size_map") {
    return true;
  }
  const text = `${image.label} ${image.id}`.toLowerCase();
  return (
    text.includes("size") ||
    text.includes("dimension") ||
    text.includes("尺寸") ||
    text.includes("尺码")
  );
}

export function normalizeImageRole(role?: string): ImageRole | undefined {
  switch (role) {
    case "main":
    case "gallery":
    case "swatch":
    case "size_map":
    case "skc":
      return role;
    default:
      return undefined;
  }
}

export function hasSavedImageRole(
  finalImages?: Array<{
    url?: string;
    role?: string;
    main?: boolean;
    swatch?: boolean;
    size_map?: boolean;
  }>,
) {
  return Boolean(
    finalImages?.some(
      (image) =>
        image.main ||
        image.swatch ||
        image.size_map ||
        normalizeImageRole(image.role) !== undefined,
    ),
  );
}

export function roleLabel(role: string) {
  switch (role) {
    case "main":
      return "主图";
    case "swatch":
      return "色块图";
    case "size_map":
      return "尺寸图";
    case "skc":
      return "SKC 图";
    default:
      return "图库";
  }
}

export function moveItem(items: string[], value: string, delta: -1 | 1) {
  const next = [...items];
  const index = next.indexOf(value);
  if (index < 0) {
    return next;
  }
  const target = index + delta;
  if (target < 0 || target >= next.length) {
    return next;
  }
  [next[index], next[target]] = [next[target], next[index]];
  return next;
}
