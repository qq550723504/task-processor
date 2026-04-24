import type { SheinStudioGeneratedDesign } from "@/lib/types/shein-studio";

export function resolveGeneratedDesignSrc(design: SheinStudioGeneratedDesign) {
  return design.imageUrl || design.dataUrl || "";
}

export function hasGeneratedDesignSrc(design: SheinStudioGeneratedDesign) {
  return Boolean(resolveGeneratedDesignSrc(design));
}
