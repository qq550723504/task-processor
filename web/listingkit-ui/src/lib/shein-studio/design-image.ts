import type { SheinStudioGeneratedDesign } from "@/lib/types/shein-studio";

function normalizeListingKitUploadFetchUrl(url: string) {
  try {
    const parsed = new URL(url);
    const prefix = "/api/v1/listing-kits/uploads/files/";
    if (!parsed.pathname.startsWith(prefix)) {
      return url;
    }

    const proxiedPath = parsed.pathname.replace(
      prefix,
      "/api/listing-kits/uploads/files/",
    );
    return `${proxiedPath}${parsed.search}`;
  } catch {
    return url;
  }
}

export function resolveGeneratedDesignSrc(design: SheinStudioGeneratedDesign) {
  const imageUrl = design.imageUrl?.trim();
  if (imageUrl) {
    return normalizeListingKitUploadFetchUrl(imageUrl);
  }
  return design.dataUrl || "";
}

export function hasGeneratedDesignSrc(design: SheinStudioGeneratedDesign) {
  return Boolean(resolveGeneratedDesignSrc(design));
}
