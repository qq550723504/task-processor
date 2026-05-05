"use client";

import Link from "next/link";
import { useMemo } from "react";

import { SheinProductPickerModal } from "@/components/listingkit/shein-studio/shein-product-picker-modal";
import {
  SheinStudioStepTabs,
  type SheinStudioStepKey,
} from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import { SheinStudioWorkbenchSlot } from "@/components/listingkit/shein-studio/shein-studio-workbench-slot";
import {
  parseSelectionFromSearchParams,
  parseSheinStudioStep,
} from "@/lib/shein-studio/url-state";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import { useLiveSearchParams } from "@/lib/utils/live-search-params";

export function SheinStudioPageShell({
  activeStep,
  initialKeyword,
  initialPage,
  initialShipmentArea,
  selection,
}: {
  activeStep?: SheinStudioStepKey;
  initialKeyword?: string;
  initialPage?: number;
  initialShipmentArea?: string;
  selection?: SDSProductVariantSelection;
} = {}) {
  const searchParams = useLiveSearchParams();
  const liveKeyword = searchParams.get("keyword") ?? initialKeyword ?? "";
  const livePage = Number(searchParams.get("page") ?? initialPage ?? 1) || 1;
  const liveShipmentArea =
    searchParams.get("shipmentArea") ?? initialShipmentArea ?? "US";
  const liveSelection = useMemo(
    () => parseSelectionFromSearchParams(searchParams) ?? selection,
    [searchParams, selection],
  );
  const liveStep = searchParams.get("step")
    ? parseSheinStudioStep(searchParams.get("step"))
    : activeStep ?? (liveSelection ? "generate" : "select");
  const hasSelection = Boolean(liveSelection?.variantId);
  const visibleStep = hasSelection ? liveStep : "select";
  const selectedVariantKey =
    liveSelection?.selectedVariantIds?.join(",") ??
    liveSelection?.variants?.map((variant) => variant.variantId).join(",") ??
    "";
  const workbenchKey = `${liveSelection?.variantId ?? 0}:${liveSelection?.prototypeGroupId ?? 0}:${liveSelection?.layerId ?? ""}:${selectedVariantKey}`;
  const stepCopy = {
    select: {
      title: "先选择要处理的 SDS 商品",
      description:
        "完成选品后，系统会带着模板和变体信息进入图片生成。",
    },
    generate: {
      title: "生成并整理图片结果",
      description:
        "这里会根据选中的 SDS 商品生成图片，完成后继续进入审核步骤。",
    },
    review: {
      title: "确认图片和资料是否可用",
      description:
        "审核通过后，系统会把当前结果带入 SHEIN 资料确认工作台。",
    },
    tasks: {
      title: "确认 SHEIN 上架资料",
      description:
        "这一步会把已确认的资料带入正式任务，继续保存草稿或提交发布。",
    },
  }[visibleStep];

  return (
    <div className="relative isolate flex-1 overflow-hidden bg-[radial-gradient(circle_at_top_left,_rgba(251,146,60,0.18),_transparent_26%),radial-gradient(circle_at_top_right,_rgba(236,72,153,0.14),_transparent_24%),linear-gradient(180deg,_#fffdf9_0%,_#f7f3ee_46%,_#efebe4_100%)]">
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(24,24,27,0.032)_1px,transparent_1px),linear-gradient(90deg,rgba(24,24,27,0.032)_1px,transparent_1px)] bg-[size:30px_30px] opacity-40" />
      <div className="relative mx-auto flex w-full max-w-7xl flex-1 flex-col gap-8 px-6 py-10 lg:px-10">
        <section className="grid gap-5 rounded-[2rem] border border-white/70 bg-white/72 px-5 py-5 shadow-[0_20px_80px_rgba(24,24,27,0.08)] backdrop-blur md:grid-cols-[1.25fr_0.75fr] lg:px-6">
          <div className="space-y-4">
            <p className="text-[11px] font-semibold uppercase tracking-[0.34em] text-rose-700">
              SHEIN 工作室
            </p>
            <div className="space-y-2">
              <h1 className="max-w-3xl font-serif text-3xl leading-tight tracking-[-0.04em] text-zinc-950 md:text-4xl">
                从 SDS 商品生成 SHEIN 上架任务
              </h1>
              <p className="max-w-2xl text-sm leading-7 text-zinc-600 md:text-base">
                按步骤选择 SDS 商品、生成款式图、审核图片，然后创建 SHEIN
                资料确认任务。
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <Link
                href="/listing-kits/shein/gallery"
                className="inline-flex h-10 items-center justify-center rounded-xl bg-zinc-950 px-4 text-sm font-medium text-white transition hover:bg-zinc-800"
                prefetch={false}
              >
                查看款式图库
              </Link>
              <Link
                href="/listing-kits?platform=shein"
                className="inline-flex h-10 items-center justify-center rounded-xl border border-zinc-200 bg-white px-4 text-sm font-medium text-zinc-900 transition hover:bg-zinc-50"
                prefetch={false}
              >
                查看 SHEIN 任务
              </Link>
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-3 md:grid-cols-1">
            <MetricCard label="发货地" value={liveShipmentArea} dark />
            <MetricCard
              label="变体数"
              value={
                liveSelection?.selectedVariantIds?.length
                  ? String(liveSelection.selectedVariantIds.length)
                  : liveSelection?.variants?.length
                    ? String(liveSelection.variants.length)
                    : liveSelection?.variantId
                      ? "1"
                      : "未选择"
              }
            />
            <MetricCard
              label="印刷区域"
              value={
                liveSelection?.printableWidth && liveSelection?.printableHeight
                  ? `${liveSelection.printableWidth}×${liveSelection.printableHeight}`
                  : "自动"
              }
            />
          </div>
        </section>

        <SheinStudioStepTabs
          activeStep={visibleStep}
          hasSelection={hasSelection}
        />

        <section className="rounded-[1.75rem] border border-white/70 bg-white/78 p-5 shadow-[0_18px_60px_rgba(24,24,27,0.06)] backdrop-blur">
          <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
            当前步骤
          </p>
          <h2 className="mt-2 text-xl font-semibold text-zinc-950">
            {stepCopy.title}
          </h2>
          <p className="mt-2 max-w-3xl text-sm leading-7 text-zinc-600">
            {stepCopy.description}
          </p>
        </section>

        <div className="space-y-6">
          {visibleStep === "select" ? (
            <SheinProductPickerModal
              initialKeyword={liveKeyword}
              initialPage={livePage}
              initialShipmentArea={liveShipmentArea}
              selection={liveSelection}
            />
          ) : null}
          {hasSelection ? (
            <SheinStudioWorkbenchSlot
              activeStep={visibleStep}
              selection={liveSelection}
              workbenchKey={workbenchKey}
            />
          ) : null}
        </div>
      </div>
    </div>
  );
}

function MetricCard({
  label,
  value,
  dark = false,
}: {
  label: string;
  value: string;
  dark?: boolean;
}) {
  return (
    <div
      className={
        dark
          ? "rounded-[1.5rem] border border-zinc-200/80 bg-zinc-950 px-5 py-4 text-white shadow-sm"
          : "rounded-[1.5rem] border border-zinc-200/80 bg-white px-5 py-4 shadow-sm"
      }
    >
      <div
        className={
          dark
            ? "text-[11px] uppercase tracking-[0.28em] text-zinc-400"
            : "text-[11px] uppercase tracking-[0.28em] text-zinc-400"
        }
      >
        {label}
      </div>
      <div className="mt-3 text-2xl font-semibold">{value}</div>
    </div>
  );
}
