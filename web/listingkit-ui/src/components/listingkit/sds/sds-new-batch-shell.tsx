import { SDSProductBrowser } from "@/components/listingkit/sds/sds-product-browser";
import { SdsRouteHeader } from "@/components/listingkit/sds/sds-route-header";

export function SdsNewBatchShell() {
  return (
    <section className="mx-auto flex w-full max-w-7xl flex-1 flex-col gap-5 px-4 py-6 lg:px-6">
      <SdsRouteHeader
        description="这里先专注完成选品；选好后再进入专门的批次工作台继续生成、审核和创建任务。"
        eyebrow="第 1 步 · 新建批次"
        links={[{ href: "/listing-kits/sds", label: "返回最近批次首页" }]}
        title="选择底版商品和子 SKU"
      />

      <SDSProductBrowser />
    </section>
  );
}
