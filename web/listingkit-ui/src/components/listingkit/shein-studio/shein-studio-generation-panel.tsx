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
import { Button } from "@/components/shared/button";
import type { SheinStudioSelectableSDSImage } from "@/lib/shein-studio/sds-selectable-images";
import type {
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
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
  selectionReady,
  variationIntensity,
  setImageStrategy,
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
  selectionReady: boolean;
  variationIntensity: SheinStudioVariationIntensity;
  setImageStrategy: (value: SheinStudioImageStrategy) => void;
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
        <div className="rounded-full bg-zinc-100 px-3 py-1 text-xs font-semibold text-zinc-600">
          {selectionReady ? "商品已选择" : "请先选择商品"}
        </div>
      </div>

      <div className="grid gap-4">
        <ArtworkGenerationSettings
          artworkModel={artworkModel}
          prompt={prompt}
          promptInputRef={promptInputRef}
          setArtworkModel={setArtworkModel}
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
      />

      {selectionReady ? (
        <div className="flex flex-wrap gap-3">
          <Button disabled={isGenerating} onClick={onGenerate}>
            {isGenerating ? "生成中..." : "生成款式图"}
          </Button>
          <Button disabled={isGenerating || isCreatingTasks} onClick={onSaveBatch} tone="ghost">
            保存批次
          </Button>
          <Button
            disabled={
              isGenerating ||
              isCreatingTasks ||
              selectedStyleCount === 0 ||
              !selectionReady
            }
            onClick={onCreateTasks}
            tone="secondary"
          >
            {isCreatingTasks ? "正在生成 SHEIN 资料..." : "生成 SHEIN 资料"}
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
