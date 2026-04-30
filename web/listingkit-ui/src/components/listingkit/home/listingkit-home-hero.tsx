import Link from "next/link";

export function ListingKitHomeHero() {
  return (
    <section className="grid gap-6 overflow-hidden rounded-[2rem] border border-white/70 bg-[linear-gradient(145deg,rgba(255,255,255,0.94),rgba(248,246,240,0.9))] p-8 shadow-[0_28px_100px_rgba(39,39,42,0.12)] lg:grid-cols-[1.15fr_0.85fr]">
      <div className="space-y-5">
        <p className="text-[11px] font-semibold uppercase tracking-[0.34em] text-teal-700">
          ListingKit
        </p>
        <div className="space-y-3">
          <h1 className="max-w-3xl text-4xl font-semibold tracking-[-0.05em] text-zinc-950 sm:text-5xl">
            SHEIN 上架工作台
          </h1>
          <p className="max-w-2xl text-sm leading-7 text-zinc-600 sm:text-base">
            直接进入 SHEIN 工作台继续做图、补资料和提交，同时保留通用
            ListingKit 入口与最近任务恢复能力。
          </p>
        </div>
        <div className="flex flex-wrap gap-3">
          <Link
            href="/listing-kits/shein"
            className="inline-flex h-11 items-center justify-center rounded-xl bg-zinc-950 px-5 text-sm font-medium text-white transition hover:bg-zinc-800"
          >
            进入 SHEIN 工作台
          </Link>
          <Link
            href="/listing-kits/new"
            className="inline-flex h-11 items-center justify-center rounded-xl bg-white px-5 text-sm font-medium text-zinc-900 ring-1 ring-zinc-200 transition hover:bg-zinc-100"
          >
            开始新的 ListingKit 任务
          </Link>
        </div>
      </div>

      <div className="relative overflow-hidden rounded-[1.75rem] border border-zinc-200/60 bg-[linear-gradient(160deg,rgba(24,24,27,0.98),rgba(17,94,89,0.95))] p-6 text-white">
        <div className="absolute inset-y-0 right-0 w-1/2 bg-[radial-gradient(circle_at_top,rgba(251,191,36,0.3),transparent_60%)]" />
        <div className="relative space-y-5">
          <p className="text-xs font-semibold uppercase tracking-[0.22em] text-teal-200">
            Workflow Focus
          </p>
          <div className="space-y-4">
            <div className="rounded-2xl border border-white/10 bg-white/5 p-4 backdrop-blur">
              <p className="text-base font-semibold text-white">聚焦 SHEIN 上架主链路</p>
              <p className="mt-1 text-sm leading-6 text-zinc-200">
                从选品、款式图、商品图到资料提交，入口集中，切换更少。
              </p>
            </div>
            <div className="rounded-2xl border border-white/10 bg-white/5 p-4 backdrop-blur">
              <p className="text-base font-semibold text-white">最近任务直接继续</p>
              <p className="mt-1 text-sm leading-6 text-zinc-200">
                保留最近任务入口，回到当前工作区，不用重新找 task 或页面。
              </p>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
