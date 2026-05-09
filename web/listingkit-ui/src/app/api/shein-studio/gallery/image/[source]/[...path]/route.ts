import { GET as getStyleGalleryImage } from "@/app/api/style-gallery/image/[source]/[...path]/route";

export const dynamic = "force-dynamic";

export async function GET(
  request: Request,
  context: { params: Promise<{ source: string; path: string[] }> },
) {
  return getStyleGalleryImage(request, context);
}
