"use client";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils/cn";

export type SheinFlowStepState = "done" | "active" | "blocked" | "pending";

export type SheinFlowStep = {
  key: string;
  label: string;
  description: string;
  href: string;
  state: SheinFlowStepState;
  actionLabel?: string;
};

function stateLabel(state: SheinFlowStepState) {
  switch (state) {
    case "done":
      return "完成";
    case "active":
      return "当前";
    case "blocked":
      return "阻断";
    default:
      return "下一步";
  }
}

function stateClasses(state: SheinFlowStepState) {
  switch (state) {
    case "done":
      return "border-emerald-200 bg-emerald-50 text-emerald-800";
    case "active":
      return "border-zinc-950 bg-zinc-950 text-white";
    case "blocked":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-zinc-200 bg-white text-zinc-500";
  }
}

export function SheinFlowNav({
  eyebrow = "SHEIN workflow",
  title = "Follow the listing flow",
  steps,
}: {
  eyebrow?: string;
  title?: string;
  steps: SheinFlowStep[];
}) {
  if (!steps.length) {
    return null;
  }

  return (
    <nav className="rounded-[1.75rem] border border-zinc-200/80 bg-white/85 p-4 shadow-sm backdrop-blur">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.26em] text-zinc-500">
            {eyebrow}
          </p>
          <h2 className="mt-1 text-lg font-semibold tracking-tight text-zinc-950">
            {title}
          </h2>
        </div>
        <p className="max-w-xl text-sm leading-6 text-zinc-600">
          按顺序操作；卡住时直接点当前步骤进入对应区域处理。
        </p>
      </div>

      <div className="mt-4 grid gap-3 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
        {steps.map((step, index) => (
          <a
            className={cn(
              "group rounded-2xl border px-4 py-3 transition hover:-translate-y-0.5 hover:shadow-md",
              stateClasses(step.state),
            )}
            href={step.href}
            key={step.key}
          >
            <div className="flex items-start justify-between gap-3">
              <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-current/20 text-xs font-semibold">
                {index + 1}
              </span>
              <Badge
                className="rounded-full border-current/20 bg-transparent px-2 py-1 text-[10px] uppercase tracking-[0.16em] text-current"
                variant="outline"
              >
                {stateLabel(step.state)}
              </Badge>
            </div>
            <div className="mt-3 space-y-1">
              <p className="text-sm font-semibold">{step.label}</p>
              <p
                className={cn(
                  "text-xs leading-5",
                  step.state === "active" ? "text-white/75" : "text-current/70",
                )}
              >
                {step.description}
              </p>
              {step.actionLabel ? (
                <p
                  className={cn(
                    "pt-1 text-[11px] font-semibold uppercase tracking-[0.16em]",
                    step.state === "active" ? "text-white" : "text-current",
                  )}
                >
                  {step.actionLabel}
                </p>
              ) : null}
            </div>
          </a>
        ))}
      </div>
    </nav>
  );
}
