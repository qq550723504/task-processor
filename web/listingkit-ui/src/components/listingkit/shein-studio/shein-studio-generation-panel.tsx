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
  SheinStudioBatchStatusGroups,
  SheinStudioBatchItem,
  SheinStudioCreatedTask,
  SheinStudioFailedTask,
  SheinStudioGroupedImageMode,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioRejectedTask,
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
  failedTasks = [],
  generationError,
  generationNotice = "",
  failedBatchItems = [],
  groupedImageMode,
  artworkModel,
  imageStrategy,
  isCreatingTasks,
  isGenerating,
  isRetryingFailedItems = false,
  retryingFailedItemId = "",
  rejectedTasks = [],
  reusedTasks = [],
  onCreateTasks,
  onDeleteBatch,
  onGenerate,
  onLoadBatch,
  onRetryFailedItem,
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
  generateButtonLabel = "生成款式图",
  selectionReady,
  showSavedBatches = true,
  statusGroups,
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
  failedTasks?: SheinStudioFailedTask[];
  generationError: string;
  generationNotice?: string;
  failedBatchItems?: SheinStudioBatchItem[];
  groupedImageMode: SheinStudioGroupedImageMode;
  artworkModel: SheinStudioArtworkModel;
  imageStrategy: SheinStudioImageStrategy;
  isCreatingTasks: boolean;
  isGenerating: boolean;
  isRetryingFailedItems?: boolean;
  retryingFailedItemId?: string;
  rejectedTasks?: SheinStudioRejectedTask[];
  reusedTasks?: SheinStudioCreatedTask[];
  onCreateTasks: () => void;
  onDeleteBatch: (batchID: string) => void;
  onGenerate: () => void;
  onLoadBatch: (batch: SheinStudioSavedBatch) => void;
  onRetryFailedItem?: (itemId: string) => void;
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
  generateButtonLabel?: string;
  selectionReady: boolean;
  showSavedBatches?: boolean;
  statusGroups?: SheinStudioBatchStatusGroups;
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
  const generateActionLabel = isGenerating
    ? isRetryingFailedItems
      ? "重试中..."
      : "生成中..."
    : generateButtonLabel;

  return (
    <div
      id="shein-studio-generator"
      className="scroll-mt-6 space-y-4 rounded-[1.75rem] border border-zinc-200/80 bg-[linear-gradient(180deg,_#ffffff_0%,_#fbfbf8_100%)] px-5 py-5 shadow-sm"
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
            isGenerating
              ? isRetryingFailedItems
                ? "正在重试失败批次"
                : "正在生成款式图"
              : !selectionReady
              ? "请先选择商品"
              : storeRequiredMessage
                ? "需先设置批次店铺"
                : "可以生成或创建任务"
          }
        />
      </div>
      <BatchStatusGroupsSummary statusGroups={statusGroups} />
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
            : isGenerating
              ? isRetryingFailedItems
                ? "当前正在重试失败批次，仅会处理失败部分，不会重复生成已成功内容。"
                : "当前批次仍在继续生成，请等待这一轮完成后再继续操作。"
              : "当前条件已齐备，确认下方参数后即可继续生成或创建任务。"}
        </div>
      ) : null}

      {generationNotice ? (
        <div className="rounded-2xl border border-amber-200 bg-amber-50/80 px-4 py-3 text-sm text-amber-900">
          {generationNotice}
        </div>
      ) : null}

      {failedBatchItems.length > 0 ? (
        <FailedBatchItemsPanel
          items={failedBatchItems}
          onRetry={onRetryFailedItem}
          retryingItemId={retryingFailedItemId}
        />
      ) : null}

      <div
        className="grid gap-4 xl:grid-cols-[minmax(0,1.08fr)_minmax(24rem,0.92fr)] xl:items-start"
        data-testid="generation-settings-grid"
      >
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
            {generateActionLabel}
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
        <SheinCreatedTasksList
          failedTasks={failedTasks}
          rejectedTasks={rejectedTasks}
          reusedTasks={reusedTasks}
          tasks={createdTasks}
        />
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

function FailedBatchItemsPanel({
  items,
  onRetry,
  retryingItemId,
}: {
  items: SheinStudioBatchItem[];
  onRetry?: (itemId: string) => void;
  retryingItemId?: string;
}) {
  return (
    <div className="rounded-[1.5rem] border border-rose-200 bg-rose-50/80 px-4 py-4">
      <div>
        <div className="text-[10px] font-semibold uppercase tracking-[0.24em] text-rose-600">
          失败项
        </div>
        <p className="mt-1 text-xs leading-5 text-rose-900">
          这些分组可单独重试，便于按失败原因逐个恢复。
        </p>
      </div>
      <div className="mt-3 space-y-3">
        {items.map((item) => {
          const itemLabel = item.targetGroupLabel?.trim() || item.targetGroupKey;
          const isRetrying = retryingItemId === item.id;
          return (
            <div
              className="rounded-2xl border border-white/90 bg-white/90 px-4 py-3 shadow-sm"
              key={item.id}
            >
              <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div className="space-y-1">
                  <p className="text-sm font-semibold text-zinc-900">{itemLabel}</p>
                  <p className="text-xs text-zinc-500">
                    {item.selectionCount} 款商品 · {item.targetGroupKey}
                  </p>
                  <p className="text-sm text-rose-800">
                    {item.lastError?.trim() || "上游返回失败，请重试该分组。"}
                  </p>
                </div>
                <Button
                  className="w-full sm:w-auto"
                  disabled={!onRetry || Boolean(retryingItemId)}
                  onClick={() => onRetry?.(item.id)}
                  type="button"
                  variant="secondary"
                >
                  {isRetrying ? "重试中..." : "重试此项"}
                </Button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function BatchStatusGroupsSummary({
  statusGroups,
}: {
  statusGroups?: SheinStudioBatchStatusGroups;
}) {
  const groups = statusGroups?.items.filter((group) => group.count > 0) ?? [];
  if (groups.length === 0) {
    return null;
  }
  return (
    <div className="rounded-[1.5rem] border border-sky-200 bg-sky-50/70 px-4 py-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <div className="text-[10px] font-semibold uppercase tracking-[0.24em] text-sky-600">
            批量状态分组
          </div>
          <p className="mt-1 text-xs leading-5 text-sky-800">
            混合结果不会阻断可提交项，失败项可单独处理或稍后重试。
          </p>
        </div>
      </div>
      <div className="mt-3 flex flex-wrap gap-2">
        {groups.map((group) => (
          <div
            className="rounded-full border border-white/80 bg-white px-3 py-2 text-xs shadow-sm"
            key={group.key}
          >
            <span className="font-semibold text-zinc-900">{group.label}</span>
            <span className="ml-2 text-zinc-500">{group.count} 项</span>
          </div>
        ))}
      </div>
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
