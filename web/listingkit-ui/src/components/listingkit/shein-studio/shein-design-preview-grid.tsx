"use client";

import { useState } from "react";
import Image from "next/image";

import { SheinDesignLightbox } from "@/components/listingkit/shein-studio/shein-design-lightbox";
import { SheinDesignReviewNote } from "@/components/listingkit/shein-studio/shein-design-review-note";
import { Button } from "@/components/ui/button";
import { resolveGeneratedDesignSrc } from "@/lib/shein-studio/design-image";
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
  readOnly = false,
  canRegenerate = true,
  onNoteChange,
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

  return (
    <>
      <section className="space-y-4 rounded-[1.75rem] border border-zinc-200/80 bg-white px-5 py-5 shadow-sm">
        <div className="flex items-center justify-between gap-3">
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

        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {designs.map((design, index) => {
            const selected = selectedIds.includes(design.id);
            const designSrc = resolveGeneratedDesignSrc(design);
            return (
              <article
                className={`overflow-hidden rounded-[1.5rem] border transition ${
                  selected
                    ? "border-emerald-700 bg-emerald-50/60 shadow-[0_12px_30px_rgba(5,150,105,0.12)]"
                    : "border-zinc-200 bg-zinc-50/70"
                }`}
                key={design.id}
              >
                <div className="space-y-3 p-4">
                  <div className="flex items-center justify-between gap-3">
                    <div className="text-sm font-semibold text-zinc-900">
                      款式 {index + 1}
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
                      className="relative block h-auto w-full overflow-hidden rounded-[1.25rem] p-0"
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
                        src={designSrc}
                        unoptimized
                        width={1024}
                      />
                    </Button>
                    <div className="grid gap-3 sm:grid-cols-2">
                      <Button
                        className="w-full"
                        onClick={() => {
                          setActivePreviewId(design.id);
                          setActivePreviewView("design");
                        }}
                        variant="secondary"
                      >
                        查看原图
                      </Button>
                      <Button
                        className="w-full"
                        onClick={() => {
                          setActivePreviewId(design.id);
                          setActivePreviewView("mockup");
                        }}
                        variant="ghost"
                      >
                        查看效果图
                      </Button>
                    </div>
                    <div className="rounded-[1rem] border border-dashed border-zinc-200 bg-zinc-50 px-3 py-3 text-xs leading-6 text-zinc-500">
                      当前卡片只展示原始款式图。打开效果图可检查 SDS 模板上的预览效果。
                    </div>
                    <SheinDesignReviewNote
                      disabled={readOnly}
                      note={design.reviewNote}
                      onChange={
                        onNoteChange
                          ? (value) => onNoteChange(design.id, value)
                          : undefined
                      }
                    />
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
                  <span>商品图数量：{productImageCount} 张</span>
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
        key={`${activeDesign?.id ?? "none"}:${selection?.variantId ?? 0}`}
        onClose={() => setActivePreviewId("")}
        onViewChange={setActivePreviewView}
        selection={selection}
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
