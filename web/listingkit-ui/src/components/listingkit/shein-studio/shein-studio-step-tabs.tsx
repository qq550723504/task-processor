"use client";

import { usePathname } from "next/navigation";
import { replaceBrowserHistory } from "@/lib/utils/browser-history";
import { useLiveSearchParams } from "@/lib/utils/live-search-params";
import { sanitizedNavigationSearchParams } from "@/lib/utils/navigation-query";

export type SheinStudioStepKey = "select" | "generate" | "review" | "tasks";

const steps: Array<{
  key: SheinStudioStepKey;
  label: string;
  description: string;
}> = [
  {
    key: "select",
    label: "1. 选择商品",
    description: "选择 SDS 底版商品和要上架的变体。",
  },
  {
    key: "generate",
    label: "2. 生成图片",
    description: "生成款式图和商品图。",
  },
  {
    key: "review",
    label: "3. 审核款式",
    description: "确认要用于上架的款式。",
  },
  {
    key: "tasks",
    label: "4. 确认资料",
    description: "打开最终确认和提交页面。",
  },
];

export function SheinStudioStepTabs({
  activeStep,
  hasSelection,
}: {
  activeStep: SheinStudioStepKey;
  hasSelection: boolean;
}) {
  const pathname = usePathname();
  const searchParams = useLiveSearchParams();

  function hrefForStep(step: SheinStudioStepKey) {
    const params = sanitizedNavigationSearchParams(searchParams);
    params.set("step", step);
    return `${pathname}?${params.toString()}`;
  }

  return (
    <nav className="grid gap-3 rounded-[1.75rem] border border-white/70 bg-white/82 p-3 shadow-sm backdrop-blur md:grid-cols-4">
      {steps.map((step) => {
        const active = step.key === activeStep;
        const locked = step.key !== "select" && !hasSelection;
        const href = locked ? hrefForStep("select") : hrefForStep(step.key);
        return (
          <button
            aria-disabled={locked}
            aria-current={active ? "step" : undefined}
            className={[
              "rounded-[1.25rem] border px-4 py-3 transition",
              active
                ? "border-zinc-950 bg-zinc-950 text-white shadow-sm"
                : "border-zinc-200 bg-white text-zinc-900 hover:border-zinc-400",
              locked ? "pointer-events-none opacity-45" : "",
            ].join(" ")}
            key={step.key}
            onClick={() => {
              if (locked || active) {
                return;
              }
              replaceBrowserHistory(href);
            }}
            type="button"
          >
            <div className="text-sm font-semibold">{step.label}</div>
            <div
              className={[
                "mt-1 text-xs leading-5",
                active ? "text-zinc-300" : "text-zinc-500",
              ].join(" ")}
            >
              {locked ? "请先选择商品。" : step.description}
            </div>
          </button>
        );
      })}
    </nav>
  );
}
