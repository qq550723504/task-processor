import { SheinStyleGalleryPage } from "@/components/listingkit/shein-style-gallery-page";
import { buildSheinStyleGallery } from "@/lib/server/shein-style-gallery";

export const dynamic = "force-dynamic";

export default async function ListingKitSheinGalleryRoute() {
  const gallery = await buildSheinStyleGallery();
  return <SheinStyleGalleryPage initialGallery={gallery} />;
}
