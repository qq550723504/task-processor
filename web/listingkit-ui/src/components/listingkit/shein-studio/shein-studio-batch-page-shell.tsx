"use client";

import { SheinStudioWorkbench } from "@/components/listingkit/shein-studio/shein-studio-workbench";

export function SheinStudioBatchPageShell({
  batchId,
}: {
  batchId: string;
}) {
  return <SheinStudioWorkbench initialBatchId={batchId} />;
}
