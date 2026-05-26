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
  promptInputRef,
  savedBatches,
  saveMessage,
  selectedSdsImages,
  selectedStyleCount,
  createTaskButtonLabel = "生成 SHEIN 资料",
  selectionReady,
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
  setRenderSizeImagesWithSds,
  setSheinStoreId,
  setStyleCount,
  setVariationIntensity,
  setTransparentBackground,
  sheinStoreId,
  styleCount,
}: {
  availableSdsImages: SheinStudioSelectableSDSImage[];
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
  promptInputRef: RefObject<HTMLTextAreaElement | null>;
  savedBatches: SheinStudioSavedBatch[];
  saveMessage: string;
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  selectedStyleCount: number;
  createTaskButtonLabel?: string;
  selectionReady: boolean;
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
  setRenderSizeImagesWithSds: (value: boolean) => void;
  setSheinStoreId: (value: string) => void;
  setStyleCount: (value: string) => void;
  setVariationIntensity: (value: SheinStudioVariationIntensity) => void;
  setTransparentBackground: (value: boolean) => void;
  sheinStoreId: string;
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

      <div className="grid gap-4">
        <ArtworkGenerationSettings
          artworkModel={artworkModel}
          disabled={isGenerating}
          groupedImageMode={groupedImageMode}
          prompt={prompt}
          promptInputRef={promptInputRef}
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
          setSheinStoreId={setSheinStoreId}
          sheinStoreId={sheinStoreId}
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
        subscriptionBlockedMessage={subscriptionBlockedMessage}
      />

      {selectionReady ? (
        <div className="flex flex-wrap gap-3">
          <Button
            disabled={isGenerating || Boolean(subscriptionBlockedMessage)}
            onClick={onGenerate}
          >
            {isGenerating ? "生成中..." : "生成款式图"}
          </Button>
          <Button disabled={isGenerating || isCreatingTasks} onClick={onSaveBatch} variant="ghost">
            保存批次
          </Button>
          <Button
            disabled={
              isGenerating ||
              isCreatingTasks ||
              selectedStyleCount === 0 ||
              !selectionReady ||
              Boolean(subscriptionBlockedMessage)
            }
            onClick={onCreateTasks}
            variant="secondary"
          >
            {isCreatingTasks ? "正在生成 SHEIN 资料..." : createTaskButtonLabel}
          </Button>
        </div>
      ) : (
        <div className="flex flex-wrap gap-3">
          <Button disabled type="button">
            先选择商品
          </Button>
        </div>
      )}

      <div id="shein-created-tasks" className="scroll-mt-6">
        <SheinCreatedTasksList tasks={createdTasks} />
      </div>
      <SheinSavedBatchesPanel
        batches={savedBatches}
        onDelete={onDeleteBatch}
        onLoad={onLoadBatch}
      />
    </div>
  );
}
