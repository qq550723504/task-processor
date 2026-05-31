import type { RefObject } from "react";

import { SheinCreatedTasksList } from "@/components/listingkit/shein-studio/shein-created-tasks-list";
import {
  ArtworkGenerationSettings,
  ProductImageGenerationSettings,
} from "@/components/listingkit/shein-studio/shein-studio-generation-form-sections";
import {
  GenerationMessages,
  parsePositiveInteger,
} from "@/components/listingkit/shein-studio/shein-studio-generation-sections";
import { SheinSavedBatchesPanel } from "@/components/listingkit/shein-studio/shein-saved-batches-panel";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { SheinStudioSelectableSDSImage } from "@/lib/shein-studio/sds-selectable-images";
import type {
  SDSGroupedPromptHistoryEntry,
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
  SheinStudioGroupedImageMode,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioSavedBatch,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";

export function SheinStudioGenerationPanel({
  availableSdsImages,
  batchProductCount = 0,
  batchStoreLabel = "未设置",
  createdTasks,
  creatingError,
  creatingMessage,
  generationError,
  groupedImageMode,
  artworkModel,
  imageStrategy,
  isCreatingTasks,
  isGenerating,
  onCreateTasks,
  onDeleteBatch,
  onGenerate,
  onLoadBatch,
  onSaveBatch,
  productImageCount,
  productImagePrompt,
  productImagePrompts,
  renderSizeImagesWithSds,
  transparentBackground,
  prompt,
  promptHistory,
  promptInputRef,
  savedBatches,
  saveMessage,
  selectedSdsImages,
  selectedStyleCount,
  createTaskButtonLabel = "生成 SHEIN 资料",
  selectionReady,
  showSavedBatches = true,
  storeRequiredMessage,
  subscriptionBlockedMessage,
  variationIntensity,
  setImageStrategy,
  setGroupedImageMode,
  setSelectedSdsImages,
  setArtworkModel,
  setProductImageCount,
  setProductImagePrompt,
  setProductImagePrompts,
  setPrompt,
  onRestorePrompt,
  setRenderSizeImagesWithSds,
  setStyleCount,
  setVariationIntensity,
  setTransparentBackground,
  styleCount,
}: {
  availableSdsImages: SheinStudioSelectableSDSImage[];
  batchProductCount?: number;
  batchStoreLabel?: string;
  createdTasks: SheinStudioCreatedTask[];
  creatingError: string;
  creatingMessage: string;
  generationError: string;
  groupedImageMode: SheinStudioGroupedImageMode;
  artworkModel: SheinStudioArtworkModel;
  imageStrategy: SheinStudioImageStrategy;
  isCreatingTasks: boolean;
  isGenerating: boolean;
  onCreateTasks: () => void;
  onDeleteBatch: (batchID: string) => void;
  onGenerate: () => void;
  onLoadBatch: (batch: SheinStudioSavedBatch) => void;
  onSaveBatch: () => void;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  renderSizeImagesWithSds: boolean;
  transparentBackground: boolean;
  prompt: string;
  promptHistory: SDSGroupedPromptHistoryEntry[];
  promptInputRef: RefObject<HTMLTextAreaElement | null>;
  savedBatches: SheinStudioSavedBatch[];
  saveMessage: string;
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  selectedStyleCount: number;
  createTaskButtonLabel?: string;
  selectionReady: boolean;
  showSavedBatches?: boolean;
  storeRequiredMessage: string;
  subscriptionBlockedMessage: string;
  variationIntensity: SheinStudioVariationIntensity;
  setImageStrategy: (value: SheinStudioImageStrategy) => void;
  setGroupedImageMode: (value: SheinStudioGroupedImageMode) => void;
  setSelectedSdsImages: (value: SheinStudioSelectedSDSImage[]) => void;
  setArtworkModel: (value: SheinStudioArtworkModel) => void;
  setProductImageCount: (value: string) => void;
  setProductImagePrompt: (value: string) => void;
  setProductImagePrompts: (value: SheinStudioProductImagePrompt[]) => void;
  setPrompt: (value: string) => void;
  onRestorePrompt: (value: string) => void;
  setRenderSizeImagesWithSds: (value: boolean) => void;
  setStyleCount: (value: string) => void;
  setVariationIntensity: (value: SheinStudioVariationIntensity) => void;
  setTransparentBackground: (value: boolean) => void;
  styleCount: string;
}) {
  const hasSdsSizeReferenceImages = availableSdsImages.some(
    (item) => item.kind === "size_reference",
  );
  const showRenderSizeImagesWithSdsOption =
    hasSdsSizeReferenceImages && imageStrategy !== "sds_official";
  const showVariationIntensity = parsePositiveInteger(styleCount) > 1;

  return (
    <div
      id="shein-studio-generator"
      className="scroll-mt-6 space-y-4 rounded-[1.75rem] border border-zinc-200/80 bg-white px-5 py-5 shadow-sm"
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
            第 2 步 · 生成
          </p>
          <h2 className="mt-1 font-serif text-2xl tracking-[-0.03em] text-zinc-950">
            款式图和商品图设置
          </h2>
        </div>
        <Badge className="rounded-full px-3 py-1 text-xs" variant="neutral">
          {selectionReady ? "商品已选择" : "请先选择商品"}
        </Badge>
      </div>

      <div
        className={`grid gap-3 rounded-[1.5rem] border px-4 py-4 sm:grid-cols-2 xl:grid-cols-3 ${
          storeRequiredMessage
            ? "border-amber-200 bg-amber-50/70"
            : "border-zinc-200 bg-zinc-50/80"
        }`}
      >
        <ExecutionMetric label="批次店铺" value={batchStoreLabel} />
        <ExecutionMetric label="批次商品" value={`${batchProductCount} 款`} />
        <ExecutionMetric
          label="当前可执行状态"
          value={
            !selectionReady
              ? "请先选择商品"
              : storeRequiredMessage
                ? "需先设置批次店铺"
                : "可以生成或创建任务"
          }
        />
      </div>
      {selectionReady ? (
        <div
          className={`rounded-2xl border px-4 py-3 text-sm ${
            storeRequiredMessage
              ? "border-amber-200 bg-amber-50/70 text-amber-800"
              : "border-emerald-200 bg-emerald-50/70 text-emerald-800"
          }`}
        >
          {storeRequiredMessage
            ? "先回到上方选择批次店铺，再继续生成。"
            : "当前条件已齐备，确认下方参数后即可继续生成或创建任务。"}
        </div>
      ) : null}

      <div className="grid gap-4">
        <ArtworkGenerationSettings
          artworkModel={artworkModel}
          disabled={isGenerating}
          groupedImageMode={groupedImageMode}
          prompt={prompt}
          promptHistory={promptHistory}
          promptInputRef={promptInputRef}
          restorePrompt={onRestorePrompt}
          setArtworkModel={setArtworkModel}
          setGroupedImageMode={setGroupedImageMode}
          setPrompt={setPrompt}
          setStyleCount={setStyleCount}
          setTransparentBackground={setTransparentBackground}
          setVariationIntensity={setVariationIntensity}
          showVariationIntensity={showVariationIntensity}
          styleCount={styleCount}
          transparentBackground={transparentBackground}
          variationIntensity={variationIntensity}
        />
        <ProductImageGenerationSettings
          availableSdsImages={availableSdsImages}
          imageStrategy={imageStrategy}
          productImageCount={productImageCount}
          productImagePrompt={productImagePrompt}
          productImagePrompts={productImagePrompts}
          renderSizeImagesWithSds={renderSizeImagesWithSds}
          selectedSdsImages={selectedSdsImages}
          setImageStrategy={setImageStrategy}
          setProductImageCount={setProductImageCount}
          setProductImagePrompt={setProductImagePrompt}
          setProductImagePrompts={setProductImagePrompts}
          setRenderSizeImagesWithSds={setRenderSizeImagesWithSds}
          setSelectedSdsImages={setSelectedSdsImages}
          showRenderSizeImagesWithSdsOption={showRenderSizeImagesWithSdsOption}
        />
      </div>

      <GenerationMessages
        creatingError={creatingError}
        creatingMessage={creatingMessage}
        generationError={generationError}
        saveMessage={saveMessage}
        selectedStyleCount={selectedStyleCount}
        selectionReady={selectionReady}
        storeRequiredMessage={storeRequiredMessage}
        subscriptionBlockedMessage={subscriptionBlockedMessage}
      />

      {selectionReady ? (
        <div className="flex flex-col gap-3 border-t border-zinc-100 pt-4 sm:flex-row sm:flex-wrap">
          <Button
            className="w-full sm:w-auto"
            disabled={
              isGenerating ||
              Boolean(subscriptionBlockedMessage) ||
              Boolean(storeRequiredMessage)
            }
            onClick={onGenerate}
          >
            {isGenerating ? "生成中..." : "生成款式图"}
          </Button>
          <Button className="w-full sm:w-auto" disabled={isGenerating || isCreatingTasks} onClick={onSaveBatch} variant="ghost">
            保存批次
          </Button>
          <Button
            className="w-full sm:w-auto"
            disabled={
              isGenerating ||
              isCreatingTasks ||
              selectedStyleCount === 0 ||
              !selectionReady ||
              Boolean(subscriptionBlockedMessage) ||
              Boolean(storeRequiredMessage)
            }
            onClick={onCreateTasks}
            variant="secondary"
          >
            {isCreatingTasks ? "正在生成 SHEIN 资料..." : createTaskButtonLabel}
          </Button>
        </div>
      ) : (
        <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap">
          <Button className="w-full sm:w-auto" disabled type="button">
            先选择商品
          </Button>
        </div>
      )}

      <div id="shein-created-tasks" className="scroll-mt-6">
        <SheinCreatedTasksList tasks={createdTasks} />
      </div>
      {showSavedBatches ? (
        <SheinSavedBatchesPanel
          batches={savedBatches}
          onDelete={onDeleteBatch}
          onLoad={onLoadBatch}
        />
      ) : null}
    </div>
  );
}

function ExecutionMetric({
  label,
  value,
}: {
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-2xl border border-white/80 bg-white px-3 py-3 shadow-sm">
      <div className="text-[10px] uppercase tracking-[0.2em] text-zinc-400">
        {label}
      </div>
      <div className="mt-1 text-sm font-semibold text-zinc-900">{value}</div>
    </div>
  );
}
