import { SheinStudioBatchPageShell } from "@/components/listingkit/shein-studio/shein-studio-batch-page-shell";

export const dynamic = "force-static";

export default function SdsBatchPage({
  params,
}: {
  params: { batchId: string };
}) {
  return <SheinStudioBatchPageShell batchId={params.batchId} />;
}
