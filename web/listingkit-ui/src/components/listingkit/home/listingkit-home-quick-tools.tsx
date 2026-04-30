import Link from "next/link";

import { Card } from "@/components/shared/card";

const QUICK_TOOLS = [
  {
    eyebrow: "Priority",
    title: "SHEIN 工作台",
    description: "进入 SHEIN 批次工作流，继续款式图、商品图和资料处理。",
    href: "/listing-kits/shein",
  },
  {
    eyebrow: "Catalog",
    title: "SDS 选品",
    description: "直接浏览 SDS 商品、变体和印刷区，快速回到素材准备阶段。",
    href: "/listing-kits/sds",
  },
  {
    eyebrow: "Control",
    title: "任务列表",
    description: "查看最近任务、状态与恢复入口，补上手动定位 task 的兜底路径。",
    href: "/listing-kits",
  },
];

export function ListingKitHomeQuickTools() {
  return (
    <section className="grid gap-4 md:grid-cols-3">
      {QUICK_TOOLS.map((tool) => (
        <Link
          key={tool.href}
          href={tool.href}
          aria-label={tool.title}
          className="group block"
        >
          <Card className="h-full rounded-[1.5rem] border-white/70 bg-white/82 p-5 shadow-[0_18px_44px_rgba(39,39,42,0.07)] transition duration-200 group-hover:-translate-y-1 group-hover:shadow-[0_28px_60px_rgba(39,39,42,0.12)]">
            <div className="space-y-3">
              <p className="text-[11px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
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
