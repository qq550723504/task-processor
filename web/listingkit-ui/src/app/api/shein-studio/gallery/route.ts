import { GET as getStyleGallery } from "@/app/api/style-gallery/route";

export const dynamic = "force-dynamic";

export async function GET() {
  return getStyleGallery();
}
