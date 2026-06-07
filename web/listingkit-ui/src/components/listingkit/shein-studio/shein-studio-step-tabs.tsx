"use client";

import { usePathname } from "next/navigation";

import { Button } from "@/components/ui/button";
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
  layout = "studio",
}: {
  activeStep: SheinStudioStepKey;
  hasSelection: boolean;
  layout?: "compact" | "studio";
}) {
  const pathname = usePathname();
  const searchParams = useLiveSearchParams();
  const compact = layout === "compact";

  function hrefForStep(step: SheinStudioStepKey) {
    const params = sanitizedNavigationSearchParams(searchParams);
    params.set("step", step);
    return `${pathname}?${params.toString()}`;
  }

  return (
    <nav
      className={
        compact
          ? "grid gap-2 rounded-lg border border-border bg-card p-2 shadow-sm sm:grid-cols-2 xl:grid-cols-4"
          : "grid gap-3 rounded-[1.75rem] border border-border/80 bg-background/82 p-3 shadow-sm backdrop-blur sm:grid-cols-2 xl:grid-cols-4"
      }
    >
      {steps.map((step) => {
        const active = step.key === activeStep;
        const locked = step.key !== "select" && !hasSelection;
        const href = locked ? hrefForStep("select") : hrefForStep(step.key);
        return (
          <Button
            aria-disabled={locked}
            aria-current={active ? "step" : undefined}
            variant={active ? "default" : "outline"}
            className={[
              "h-auto justify-start text-left",
              compact
                ? "rounded-lg px-3 py-2"
                : "rounded-[1.25rem] px-4 py-3",
              active ? "text-white shadow-sm" : "text-foreground",
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
                compact ? "mt-0.5 text-xs leading-5" : "mt-1 text-xs leading-5",
                active ? "text-zinc-300" : "text-muted-foreground",
              ].join(" ")}
            >
              {locked ? "请先选择商品。" : step.description}
            </div>
          </Button>
        );
      })}
    </nav>
  );
}
