import Link from "next/link";

import { Card } from "@/components/shared/card";

const QUICK_TOOLS = [
  {
    eyebrow: "Create",
    title: "新建 ListingKit 任务",
    description: "从 1688 链接、图片或文本开始生成标准商品。",
    href: "/listing-kits/new",
  },
  {
    eyebrow: "Source",
    title: "POD",
    description: "选择 POD 商品、子 SKU 和印刷区，创建后续生成任务。",
    href: "/listing-kits/sds",
  },
  {
    eyebrow: "Canonical",
    title: "标准商品",
    description: "查看已抽取的标准商品事实和来源任务。",
    href: "/listing-kits/canonical-products",
  },
  {
    eyebrow: "Control",
    title: "任务列表",
    description: "恢复任务、查看状态，继续审核或排障。",
    href: "/listing-kits",
  },
];

export function ListingKitHomeQuickTools() {
  return (
    <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
      {QUICK_TOOLS.map((tool) => (
        <Link
          key={tool.href}
          href={tool.href}
          aria-label={tool.title}
          className="group block"
        >
          <Card className="h-full rounded-lg border-zinc-200 bg-white p-5 shadow-sm transition duration-200 group-hover:-translate-y-0.5 group-hover:shadow-md">
            <div className="space-y-3">
              <p className="text-[11px] font-semibold uppercase text-zinc-500">
                {tool.eyebrow}
              </p>
              <div className="space-y-2">
                <p className="text-lg font-semibold text-zinc-950">{tool.title}</p>
                <p className="text-sm leading-6 text-zinc-600">
                  {tool.description}
                </p>
              </div>
            </div>
          </Card>
        </Link>
      ))}
    </section>
  );
}
