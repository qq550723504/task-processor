import { redirect } from "next/navigation";

export default async function ListingKitSheinBatchPage({
  params,
}: {
  params: Promise<{ batchId: string }>;
}) {
  const { batchId } = await params;

  redirect(`/listing-kits/sds/batches/${batchId}`);
}
