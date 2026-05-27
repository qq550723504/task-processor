import { SDSProductBrowser } from "@/components/listingkit/sds/sds-product-browser";
import { SdsRouteHeader } from "@/components/listingkit/sds/sds-route-header";

export function SdsNewBatchShell({
  isQuickSingleEntry = false,
}: {
  isQuickSingleEntry?: boolean;
}) {
  return (
    <section className="mx-auto flex w-full max-w-7xl flex-1 flex-col gap-5 px-4 py-6 lg:px-6">
      <SdsRouteHeader
        description="这里先专注完成选品；选好后再进入专门的批次工作台继续生成、审核和创建任务。"
        eyebrow="第 1 步 · 新建批次"
        links={[{ href: "/listing-kits/sds", label: "返回最近批次首页" }]}
        title="选择底版商品和子 SKU"
      />

      {isQuickSingleEntry ? (
        <div className="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-900">
          当前是快速单个生成路径：选 1 个商品后就可以直接进入批次工作台开始生成。
        </div>
      ) : null}

      <SDSProductBrowser />
    </section>
  );
}
