import type { SheinStudioGeneratedDesign } from "@/lib/types/shein-studio";

function normalizeListingKitUploadFetchUrl(url: string) {
  const prefix = "/api/v1/listing-kits/uploads/files/";
  if (url.startsWith(prefix)) {
    return url.replace(prefix, "/api/listing-kits/uploads/files/");
  }

  try {
    const parsed = new URL(url);
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

export function resolveGeneratedDesignOriginalSrc(
  design: SheinStudioGeneratedDesign,
) {
  return resolveGeneratedDesignSrc({
    ...design,
    imageUrl: design.originalImageUrl || design.imageUrl,
  });
}

export function resolveGeneratedDesignFinalSrc(
  design: SheinStudioGeneratedDesign,
) {
  const imageUrl = design.imageUrl?.trim();
  if (design.backgroundRemovalStatus !== "succeeded" || !imageUrl) {
    return "";
  }

  return normalizeListingKitUploadFetchUrl(imageUrl);
}

export function hasGeneratedDesignSrc(design: SheinStudioGeneratedDesign) {
  return Boolean(resolveGeneratedDesignSrc(design));
}
