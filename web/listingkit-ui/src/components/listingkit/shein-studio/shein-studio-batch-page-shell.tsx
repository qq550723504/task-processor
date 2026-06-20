"use client";

import { SheinStudioWorkbench } from "@/components/listingkit/shein-studio/shein-studio-workbench";

export function SheinStudioBatchPageShell({
  batchId,
}: {
  batchId: string;
}) {
  return (
    <section className="flex flex-1 flex-col bg-background">
      <div className="flex w-full max-w-none flex-1 flex-col gap-4 px-4 py-6 lg:px-6 xl:px-8">
        <SheinStudioWorkbench initialBatchId={batchId} />
      </div>
    </section>
  );
}
