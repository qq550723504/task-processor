"use client";

import { useState } from "react";
import Image from "next/image";

import { SheinDesignLightbox } from "@/components/listingkit/shein-studio/shein-design-lightbox";
import { Button } from "@/components/ui/button";
import { resolveGeneratedDesignSrc } from "@/lib/shein-studio/design-image";
import { toThumbnailPreviewUrl } from "@/lib/utils/imgproxy-url";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
} from "@/lib/types/shein-studio";

export function SheinDesignPreviewGrid({
  designs,
  selectedIds,
  onToggle,
  onRegenerate,
  onBackToGenerate,
  regeneratingId,
  selection,
  selectionByTargetGroupKey,
  readOnly = false,
  canRegenerate = true,
  onCreateReviewTasks,
  isCreatingTasks = false,
  createActionLabel = "为已批准款式生成 SHEIN 资料",
  createActionDisabledReason,
  imageStrategy,
  productImageCount,
  renderSizeImagesWithSds,
}: {
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  onToggle: (designId: string) => void;
  onRegenerate: (designId: string) => void;
  onBackToGenerate?: () => void;
  regeneratingId?: string;
  selection?: SDSProductVariantSelection;
  selectionByTargetGroupKey?: Map<string, SDSProductVariantSelection>;
  readOnly?: boolean;
  canRegenerate?: boolean;
  onNoteChange?: (designId: string, note: string) => void;
  onCreateReviewTasks?: () => void;
  isCreatingTasks?: boolean;
  createActionLabel?: string;
  createActionDisabledReason?: string;
  imageStrategy: SheinStudioImageStrategy;
  productImageCount: string;
  renderSizeImagesWithSds: boolean;
}) {
  const [activePreviewId, setActivePreviewId] = useState<string>("");
  const [activePreviewView, setActivePreviewView] = useState<"design" | "mockup">(
    "design",
  );

  if (designs.length === 0) {
    return null;
  }

  const activeDesign = designs.find((item) => item.id === activePreviewId) ?? null;
  const activeSelectionForPreview =
    (activeDesign?.targetGroupKey
      ? selectionByTargetGroupKey?.get(activeDesign.targetGroupKey)
      : undefined) ?? selection;

  return (
    <>
      <section className="space-y-4 rounded-[1.75rem] border border-zinc-200/80 bg-[linear-gradient(180deg,_#ffffff_0%,_#f8faf7_100%)] px-5 py-5 shadow-sm">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
              已生成款式
            </p>
            <h2 className="mt-1 font-serif text-2xl tracking-[-0.03em] text-zinc-950">
              先审核每个款式，再生成 SHEIN 资料。
            </h2>
          </div>
          <div className="text-sm text-zinc-500">
            已选 {selectedIds.length} / {designs.length}
          </div>
        </div>

        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
          {designs.map((design, index) => {
            const selected = selectedIds.includes(design.id);
            const designSrc = resolveGeneratedDesignSrc(design);
            const designThumbSrc = toThumbnailPreviewUrl(designSrc, {
              width: 720,
              height: 720,
            });
            return (
              <article
                className={`overflow-hidden rounded-[1.5rem] border transition hover:-translate-y-0.5 hover:shadow-[0_18px_45px_rgba(24,24,27,0.10)] ${
                  selected
                    ? "border-emerald-700 bg-emerald-50/60 shadow-[0_12px_30px_rgba(5,150,105,0.12)]"
                    : "border-zinc-200 bg-zinc-50/70"
                }`}
                key={design.id}
              >
                <div className="space-y-3 p-4">
                  <div className="flex items-center justify-between gap-3">
                    <div className="min-w-0">
                      <div className="text-sm font-semibold text-zinc-900">
                        款式 {index + 1}
                      </div>
                      {design.targetGroupLabel ? (
                        <div className="mt-1 text-xs font-medium text-zinc-500">
                          {design.targetGroupLabel}
                        </div>
                      ) : null}
                    </div>
                    <div className="flex items-center gap-2">
                      <div
                        className={`rounded-full px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${
                          selected
                            ? "bg-emerald-100 text-emerald-800"
                            : "bg-zinc-200 text-zinc-600"
                        }`}
                      >
                        {selected ? "已选中" : "未选中"}
                      </div>
                      {readOnly ? (
                        <div
                          className={`rounded-xl px-3 py-2 text-xs font-semibold ${
                            selected
                              ? "bg-emerald-700 text-white"
                              : "bg-zinc-200 text-zinc-700"
                          }`}
                        >
                          {selected ? "已批准" : "未批准"}
                        </div>
                      ) : (
                        <Button
                          onClick={() => onToggle(design.id)}
                          variant={selected ? "primary" : "secondary"}
                        >
                          {selected ? "取消批准" : "批准"}
                        </Button>
                      )}
                    </div>
                  </div>
                  {readOnly || !canRegenerate ? null : (
                    <div className="flex gap-3">
                      <Button
                        className="flex-1"
                        onClick={() => onRegenerate(design.id)}
                        variant="ghost"
                      >
                        {regeneratingId === design.id ? "重新生成中..." : "重新生成"}
                      </Button>
                    </div>
                  )}

                  <div className="space-y-3">
                    <Button
                      className="relative block aspect-square h-auto w-full overflow-hidden rounded-[1.25rem] border-zinc-200 bg-zinc-950/5 p-0"
                      onClick={() => {
                        setActivePreviewId(design.id);
                        setActivePreviewView("design");
                      }}
                      type="button"
                      variant="outline"
                    >
                      <Image
                        alt={`生成款式 ${index + 1}`}
                        className="h-full w-full object-cover"
                        height={1024}
                        src={designThumbSrc || designSrc}
                        unoptimized
                        width={1024}
                      />
                    </Button>
                  </div>
                </div>
              </article>
            );
          })}
        </div>

        {!readOnly && onCreateReviewTasks ? (
          <div className="flex flex-wrap items-center justify-between gap-4 rounded-[1.35rem] border border-emerald-200 bg-emerald-50 px-4 py-4">
            <div className="space-y-3">
              <div className="text-sm font-semibold text-emerald-950">
                已批准 {selectedIds.length} 个款式
              </div>
              <div className="mt-1 text-sm leading-6 text-emerald-800">
                {createActionDisabledReason ||
                  "下一步：上传已批准款式，生成商品图并创建 SHEIN 审核工作区。"}
              </div>
              <div className="rounded-2xl border border-emerald-300/80 bg-white/65 px-3 py-3 text-sm text-emerald-950">
                <div className="font-semibold">当前商品图设置</div>
                <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-sm leading-6 text-emerald-900">
                  <span>商品图方式：{formatImageStrategyLabel(imageStrategy)}</span>
                  <span>
                    商品图数量：
                    {imageStrategy === "sds_official"
                      ? "使用全部 SDS 图"
                      : `${productImageCount} 张`}
                  </span>
                  <span>
                    尺寸图：{renderSizeImagesWithSds ? "使用 SDS 渲染" : "不额外使用 SDS 渲染"}
                  </span>
                </div>
              </div>
            </div>
            <div className="flex flex-wrap gap-3">
              {onBackToGenerate ? (
                <Button onClick={onBackToGenerate} variant="ghost">
                  修改商品图设置
                </Button>
              ) : null}
              <Button
                disabled={
                  isCreatingTasks ||
                  selectedIds.length === 0 ||
                  Boolean(createActionDisabledReason)
                }
                onClick={onCreateReviewTasks}
              >
                {isCreatingTasks ? "正在生成 SHEIN 资料..." : createActionLabel}
              </Button>
            </div>
          </div>
        ) : null}
      </section>
      <SheinDesignLightbox
        activeView={activePreviewView}
        design={activeDesign}
        key={`${activeDesign?.id ?? "none"}:${activeSelectionForPreview?.variantId ?? 0}`}
        onClose={() => setActivePreviewId("")}
        onViewChange={setActivePreviewView}
        selection={activeSelectionForPreview}
      />
    </>
  );
}

function formatImageStrategyLabel(strategy: SheinStudioImageStrategy) {
  switch (strategy) {
    case "sds_official":
      return "SDS 官方渲染";
    case "hybrid":
      return "混合生成";
    case "ai_generated":
    default:
      return "AI 生成商品图";
  }
}
