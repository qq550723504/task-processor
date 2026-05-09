export {
  STYLE_GALLERY_HANDOFF_STORAGE_KEY as SHEIN_STUDIO_GALLERY_HANDOFF_STORAGE_KEY,
  buildStyleGalleryHandoff as buildSheinStudioGalleryHandoff,
  consumeStyleGalleryHandoff as consumeSheinStudioGalleryHandoff,
  evaluateSDSRatioMatch,
  saveStyleGalleryHandoff as saveSheinStudioGalleryHandoff,
  styleGalleryHandoffToDesign as galleryHandoffToDesign,
} from "@/lib/style-gallery/gallery-handoff";

export type {
  SDSRatioMatch,
  SDSRatioMatchStatus,
  StyleGalleryHandoff as SheinStudioGalleryHandoff,
} from "@/lib/style-gallery/gallery-handoff";
