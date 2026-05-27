import { SDSProductBrowser } from "@/components/listingkit/sds/sds-product-browser";

export function SdsNewBatchShell() {
  return (
    <section className="mx-auto flex w-full max-w-7xl flex-1 flex-col gap-5 px-4 py-6 lg:px-6">
      <div className="space-y-2 rounded-lg border border-zinc-200 bg-white px-5 py-5 shadow-sm">
        <p className="text-xs font-semibold uppercase tracking-[0.18em] text-emerald-700">
          第 1 步 · 新建批次
        </p>
        <h1 className="text-2xl font-semibold tracking-tight text-zinc-950">
          选择底版商品和子 SKU
        </h1>
        <p className="max-w-3xl text-sm leading-7 text-zinc-600">
          完成商品选择后，再进入专门的批次工作台继续生成和审核。
        </p>
      </div>

      <SDSProductBrowser />
    </section>
  );
}
