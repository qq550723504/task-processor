import { SheinStudioBatchPageShell } from "@/components/listingkit/shein-studio/shein-studio-batch-page-shell";

export const dynamic = "force-static";

export default async function SdsBatchPage({
  params,
}: {
  params: Promise<{ batchId: string }>;
}) {
  const { batchId } = await params;

  return <SheinStudioBatchPageShell batchId={batchId} />;
}
