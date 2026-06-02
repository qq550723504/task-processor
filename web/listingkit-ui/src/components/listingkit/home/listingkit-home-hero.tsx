import Link from "next/link";

const SOURCE_CHIPS = ["1688 链接", "图片素材", "商品文案", "SDS 商品"];

const WORKFLOW_STEPS = [
  {
    step: "01",
    title: "输入源信息",
    description: "接收 1688、用户提供的商品资料、图片或 SDS 商品数据。",
  },
  {
    step: "02",
    title: "生成标准商品",
    description: "沉淀统一的标题、类目、属性、规格、变体、图片和价格基础信息。",
  },
  {
    step: "03",
    title: "生成平台资料",
    description: "基于标准商品事实生成 SHEIN 等平台所需的上架资料包。",
  },
  {
    step: "04",
    title: "审核 / 上架",
    description: "处理阻断项、人工确认项和提交结果，最终保存草稿或发布。",
  },
];

export function ListingKitHomeHero() {
  return (
    <section className="overflow-hidden rounded-lg border border-zinc-200 bg-white p-5 shadow-sm sm:p-8">
      <div className="grid gap-7 xl:grid-cols-[0.92fr_1.08fr] xl:items-center">
        <div className="space-y-5">
          <p className="text-[11px] font-semibold uppercase text-teal-700">
            ListingKit
          </p>
          <div className="space-y-3">
            <h1 className="max-w-3xl text-3xl font-semibold text-zinc-950 sm:text-5xl">
              从商品源信息生成多平台上架资料
            </h1>
            <p className="max-w-2xl text-sm leading-7 text-zinc-600 sm:text-base">
              ListingKit 的主流程是先把 1688、图片、文案或 SDS 商品资料整理成
              标准商品，再按平台模板生成可审核、可提交的上架资料。
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            {SOURCE_CHIPS.map((chip) => (
              <span
                key={chip}
                className="rounded-full border border-zinc-200 bg-zinc-50 px-3 py-1 text-xs font-medium text-zinc-700"
              >
                {chip}
              </span>
            ))}
          </div>
          <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap">
            <Link
              href="/listing-kits/new"
              className="inline-flex h-11 w-full items-center justify-center rounded-lg bg-zinc-950 px-5 text-sm font-medium text-white transition hover:bg-zinc-800 sm:w-auto"
            >
              开始生成商品资料
            </Link>
            <Link
              href="/listing-kits/canonical-products"
              className="inline-flex h-11 w-full items-center justify-center rounded-lg bg-white px-5 text-sm font-medium text-zinc-900 ring-1 ring-zinc-200 transition hover:bg-zinc-100 sm:w-auto"
            >
              查看标准商品
            </Link>
          </div>
        </div>

        <div className="grid gap-3 sm:grid-cols-2">
          {WORKFLOW_STEPS.map((item) => (
            <div
              key={item.step}
              className="rounded-lg border border-zinc-200 bg-zinc-50 p-4"
            >
              <p className="text-xs font-semibold text-teal-700">{item.step}</p>
              <div className="mt-3 space-y-1.5">
                <p className="text-base font-semibold text-zinc-950">{item.title}</p>
                <p className="text-sm leading-6 text-zinc-600">{item.description}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
