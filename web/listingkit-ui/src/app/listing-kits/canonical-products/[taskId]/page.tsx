import { CanonicalProductDetailPage } from "@/components/listingkit/canonical/canonical-product-detail-page";

export default async function ListingKitCanonicalProductDetailRoute({
  params,
}: {
  params: Promise<{ taskId: string }>;
}) {
  const { taskId } = await params;
  return <CanonicalProductDetailPage taskId={taskId} />;
}
