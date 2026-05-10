import {
  evaluateSDSRatioMatch,
  type SDSRatioMatch,
} from "@/lib/shein-studio/gallery-handoff";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { SheinStudioGeneratedDesign } from "@/lib/types/shein-studio";

export const STUDIO_SESSION_SYNC_TIMEOUT_MS = 15_000;

export function evaluateImportedGalleryDesigns(
  designs: SheinStudioGeneratedDesign[],
  selection?: SDSProductVariantSelection,
): SDSRatioMatch | null {
  const imported = designs.find(
    (design) => design.role === "gallery" && design.sourceWidth && design.sourceHeight,
  );
  if (!imported) {
    return null;
  }
  return evaluateSDSRatioMatch({
    sourceWidth: imported.sourceWidth,
    sourceHeight: imported.sourceHeight,
    targetWidth: selection?.printableWidth,
    targetHeight: selection?.printableHeight,
  });
}
