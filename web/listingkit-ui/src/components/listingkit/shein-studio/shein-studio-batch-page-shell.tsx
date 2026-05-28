"use client";

import { SdsRouteHeader } from "@/components/listingkit/sds/sds-route-header";
import { SheinStudioWorkbench } from "@/components/listingkit/shein-studio/shein-studio-workbench";

export function SheinStudioBatchPageShell({
  batchId,
}: {
  batchId: string;
}) {
  return (
    <section className="flex flex-1 flex-col bg-zinc-50">
      <div className="flex w-full flex-col gap-3 pt-6">
        <SdsRouteHeader
          description={`当前正在继续处理批次 ${batchId}，可以在这里继续生成、审核和创建任务。`}
          eyebrow="BATCH WORKBENCH"
          links={[
            { href: "/listing-kits/sds", label: "返回最近批次首页" },
            {
              href: `/listing-kits/sds/new?targetBatchId=${batchId}`,
              label: "去 SDS 选品并加入当前批次",
            },
          ]}
          title="批次工作台"
        />
      </div>

      <SheinStudioWorkbench initialBatchId={batchId} />
    </section>
  );
}
