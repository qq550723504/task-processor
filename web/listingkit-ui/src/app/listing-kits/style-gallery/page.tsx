import { StyleGalleryPage } from "@/components/listingkit/style-gallery/style-gallery-page";
import { buildStyleGallery } from "@/lib/server/style-gallery";

export const dynamic = "force-dynamic";

export default async function ListingKitStyleGalleryRoute() {
  const gallery = await buildStyleGallery();
  return <StyleGalleryPage initialGallery={gallery} />;
}
