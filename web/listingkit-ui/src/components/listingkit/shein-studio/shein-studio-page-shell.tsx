import Link from "next/link";

import {
  SheinFlowNav,
  type SheinFlowStep,
} from "@/components/listingkit/shein/shein-flow-nav";
import { SheinProductPickerModal } from "@/components/listingkit/shein-studio/shein-product-picker-modal";
import { SheinStudioWorkbenchSlot } from "@/components/listingkit/shein-studio/shein-studio-workbench-slot";
import type { SDSProductVariantSelection } from "@/lib/types/sds";

export function SheinStudioPageShell({
  initialKeyword,
  initialPage,
  initialShipmentArea,
  selection,
}: {
  initialKeyword: string;
  initialPage: number;
  initialShipmentArea: string;
  selection?: SDSProductVariantSelection;
}) {
  const selectedVariantKey =
    selection?.selectedVariantIds?.join(",") ??
    selection?.variants?.map((variant) => variant.variantId).join(",") ??
    "";
  const workbenchKey = `${selection?.variantId ?? 0}:${selection?.prototypeGroupId ?? 0}:${selection?.layerId ?? ""}:${selectedVariantKey}`;
  const studioSteps: SheinFlowStep[] = [
    {
      key: "select-product",
      label: "Select SDS product",
      description: selection?.variantId
        ? `${selection.productName} · ${selection.variantLabel}`
        : "按发货地、分类和价格筛选 SDS 底版商品。",
      href: "#sds-product-browser",
      state: selection?.variantId ? "done" : "active",
      actionLabel: selection?.variantId ? "Product selected" : "Choose variant",
    },
    {
      key: "generate-style",
      label: "Generate styles",
      description: "输入主题提示词和款式数量，生成可印刷图案。",
      href: "#shein-studio-generator",
      state: selection?.variantId ? "active" : "pending",
      actionLabel: "Open generator",
    },
    {
      key: "review-style",
      label: "Review artwork",
      description: "只审核 AI 款式图，批准后才进入 SHEIN 资料生成。",
      href: "#shein-style-review",
      state: "pending",
      actionLabel: "Review batch",
    },
    {
      key: "create-review-task",
      label: "Create SHEIN task",
      description: "把批准的款式转成 ListingKit SHEIN 审核任务。",
      href: "#shein-created-tasks",
      state: "pending",
      actionLabel: "Generate SHEIN data",
    },
  ];

  return (
    <div className="relative isolate flex-1 overflow-hidden bg-[radial-gradient(circle_at_top_left,_rgba(251,146,60,0.18),_transparent_26%),radial-gradient(circle_at_top_right,_rgba(236,72,153,0.14),_transparent_24%),linear-gradient(180deg,_#fffdf9_0%,_#f7f3ee_46%,_#efebe4_100%)]">
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(24,24,27,0.032)_1px,transparent_1px),linear-gradient(90deg,rgba(24,24,27,0.032)_1px,transparent_1px)] bg-[size:30px_30px] opacity-40" />
      <div className="relative mx-auto flex w-full max-w-7xl flex-1 flex-col gap-8 px-6 py-10 lg:px-10">
        <section className="grid gap-5 rounded-[2rem] border border-white/70 bg-white/72 px-5 py-5 shadow-[0_20px_80px_rgba(24,24,27,0.08)] backdrop-blur md:grid-cols-[1.25fr_0.75fr] lg:px-6">
          <div className="space-y-4">
            <p className="text-[11px] font-semibold uppercase tracking-[0.34em] text-rose-700">
              SHEIN Studio
            </p>
            <div className="space-y-2">
              <h1 className="max-w-3xl font-serif text-3xl leading-tight tracking-[-0.04em] text-zinc-950 md:text-4xl">
                SDS product to SHEIN task.
              </h1>
              <p className="max-w-2xl text-sm leading-7 text-zinc-600 md:text-base">
                Select or change the SDS product in one place. The workbench only
                handles style generation, approval, and SHEIN data creation.
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <Link
                href="/listing-kits/shein/gallery"
                className="inline-flex h-10 items-center justify-center rounded-xl bg-zinc-950 px-4 text-sm font-medium text-white transition hover:bg-zinc-800"
              >
                View style gallery
              </Link>
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-3 md:grid-cols-1">
            <MetricCard label="Shipment area" value={initialShipmentArea} dark />
            <MetricCard
              label="Variants"
              value={
                selection?.selectedVariantIds?.length
                  ? String(selection.selectedVariantIds.length)
                  : selection?.variants?.length
                    ? String(selection.variants.length)
                    : selection?.variantId
                      ? "1"
                      : "Pending"
              }
            />
            <MetricCard
              label="Printable area"
              value={
                selection?.printableWidth && selection?.printableHeight
                  ? `${selection.printableWidth}×${selection.printableHeight}`
                  : "Auto"
              }
            />
          </div>
        </section>

        <SheinFlowNav
          steps={studioSteps}
          title="Create SHEIN listings from SDS products"
        />

        <div className="space-y-6">
          <SheinProductPickerModal
            initialKeyword={initialKeyword}
            initialPage={initialPage}
            initialShipmentArea={initialShipmentArea}
            selection={selection}
          />
          {selection?.variantId ? (
            <SheinStudioWorkbenchSlot
              selection={selection}
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
