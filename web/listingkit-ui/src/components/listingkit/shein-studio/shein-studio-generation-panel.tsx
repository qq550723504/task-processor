import Image from "next/image";
import type { ReactNode, RefObject } from "react";

import { SheinCreatedTasksList } from "@/components/listingkit/shein-studio/shein-created-tasks-list";
import { SheinSavedBatchesPanel } from "@/components/listingkit/shein-studio/shein-saved-batches-panel";
import { Button } from "@/components/shared/button";
import { DEFAULT_SHEIN_STORE_ID } from "@/lib/shein-studio/create-review-tasks";
import type { SheinStudioSelectableSDSImage } from "@/lib/shein-studio/sds-selectable-images";
import { SHEIN_STUDIO_PRODUCT_IMAGE_ROLES } from "@/lib/shein-studio/storage-shared";
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
        <div className="space-y-4 rounded-[1.5rem] border border-emerald-200 bg-[linear-gradient(135deg,_#ecfdf5,_#f8fafc)] px-4 py-4">
          <SectionHeading
            eyebrow="款式图"
            title="生成 POD 款式图"
            description="这里生成的是用于印刷的平面图案。商品场景图在下一块设置。"
          />
          <label className="space-y-2">
            <span className="text-sm font-medium text-zinc-700">
              主题提示词 <span className="text-rose-600">*</span>
            </span>
            <textarea
              className="min-h-40 w-full rounded-2xl border border-emerald-200 bg-white/80 px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-emerald-900 focus:bg-white"
              onChange={(event) => setPrompt(event.target.value)}
              placeholder="例如：美国国旗主题，复古学院风，线条清晰，适合印刷。"
              ref={promptInputRef}
              value={prompt}
            />
            <p className="text-xs leading-6 text-zinc-600">
              系统会优先生成适合 POD 印刷的图案：大面积形状、清晰对比、减少细线和过小文字。
            </p>
          </label>
          <NumberInput
            label="款式数量"
            max={5}
            min={1}
            setValue={setStyleCount}
            value={styleCount}
          />
          <label className="space-y-2">
            <span className="text-sm font-medium text-zinc-700">变化强度</span>
            <select
              className="w-full rounded-2xl border border-emerald-200 bg-white/80 px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-emerald-900 focus:bg-white"
              onChange={(event) =>
                setVariationIntensity(
                  event.target.value as SheinStudioVariationIntensity,
                )
              }
              value={variationIntensity}
            >
              <option value="light">轻变化</option>
              <option value="medium">中变化</option>
              <option value="strong">强变化</option>
            </select>
            <p className="text-xs leading-6 text-zinc-600">
              只影响款式图批量生成。系统会保持同一核心卖点和视觉风格，同时按强度拉开构图和元素差异。
            </p>
          </label>
          <div className="grid gap-4 lg:grid-cols-2">
            <label className="space-y-2">
              <span className="text-sm font-medium text-zinc-700">款式图模型</span>
              <select
                className="w-full rounded-2xl border border-emerald-200 bg-white/80 px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-emerald-900 focus:bg-white"
                onChange={(event) => {
                  const nextModel = event.target.value as SheinStudioArtworkModel;
                  setArtworkModel(nextModel);
                  if (nextModel !== "gpt-image-2" && transparentBackground) {
                    setTransparentBackground(false);
                  }
                }}
                value={transparentBackground ? "gpt-image-2" : artworkModel}
              >
                <option value="nanobanana">Nano Banana（默认）</option>
                <option value="gpt-image-2">GPT Image 2（支持透明背景）</option>
              </select>
            </label>
            <label className="flex items-start gap-3 rounded-2xl border border-emerald-200 bg-white/75 px-4 py-3">
              <input
                checked={transparentBackground}
                className="mt-1 h-4 w-4 rounded border-emerald-300 text-zinc-950"
                onChange={(event) => {
                  const checked = event.target.checked;
                  setTransparentBackground(checked);
                  if (checked) {
                    setArtworkModel("gpt-image-2");
                  }
                }}
                type="checkbox"
              />
              <span className="text-sm leading-6 text-emerald-950">
                <span className="block font-semibold">生成透明背景图案</span>
                <span className="block text-xs text-emerald-800">
                  开启后自动切换到 GPT Image 2，并要求输出真正 alpha
                  透明背景，不再让 Nano Banana 模拟透明。
                </span>
              </span>
            </label>
          </div>
        </div>

        <div className="space-y-4 rounded-[1.5rem] border border-zinc-200 bg-zinc-50 px-4 py-4">
          <SectionHeading
            eyebrow="商品图"
            title="设置上架商品图生成"
            description="审核通过的款式转成 SHEIN 资料时，会使用这里的商品图设置。"
          />
          <div className="grid gap-4 lg:grid-cols-2">
            <NumberInput
              label="商品图数量"
              max={9}
              min={1}
              setValue={setProductImageCount}
              value={productImageCount}
            />
            <label className="space-y-2">
              <span className="text-sm font-medium text-zinc-700">SHEIN 店铺 ID</span>
              <input
                className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
                inputMode="numeric"
                onChange={(event) => setSheinStoreId(event.target.value)}
                placeholder={DEFAULT_SHEIN_STORE_ID}
                value={sheinStoreId}
              />
            </label>
          </div>

          <label className="space-y-2">
            <span className="text-sm font-medium text-zinc-700">图片策略</span>
            <select
              className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
              onChange={(event) =>
                setImageStrategy(event.target.value as SheinStudioImageStrategy)
              }
              value={imageStrategy}
            >
              <option value="ai_generated">AI 生成商品图</option>
              <option value="sds_official">SDS 官方渲染</option>
              <option value="hybrid">混合：SDS 主图 + AI 图库</option>
            </select>
            <p className="text-xs leading-6 text-zinc-500">
              AI 生成模式不调用 SDS 设计器；SDS 官方渲染会使用模板图；
              混合模式先用 SDS 图，再追加 AI 商品图。
            </p>
          </label>

          {imageStrategy === "hybrid" || imageStrategy === "sds_official" ? (
            <SDSImagePicker
              availableImages={availableSdsImages}
              selectedImages={selectedSdsImages}
              setSelectedImages={setSelectedSdsImages}
            />
          ) : null}

          {showRenderSizeImagesWithSdsOption ? (
            <label className="flex items-start gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3">
              <input
                checked={renderSizeImagesWithSds}
                className="mt-1 h-4 w-4 rounded border-amber-300 text-zinc-950"
                onChange={(event) => setRenderSizeImagesWithSds(event.target.checked)}
                type="checkbox"
              />
              <span className="text-sm leading-6 text-amber-950">
                <span className="block font-semibold">尺寸图也使用 SDS 渲染</span>
                <span className="block text-xs text-amber-800">
                  AI 或混合模式下会额外调用 SDS，只取尺寸图用于 SHEIN 尺寸图，不替换主图和场景图。
                </span>
              </span>
            </label>
          ) : null}

          <label className="space-y-2">
            <span className="text-sm font-medium text-zinc-700">
              全局商品图提示词
            </span>
            <textarea
              className="min-h-24 w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
              onChange={(event) => setProductImagePrompt(event.target.value)}
              placeholder="可选。会应用到每一张商品图，例如：背景保持暖色、简洁。"
              value={productImagePrompt}
            />
            <p className="text-xs leading-6 text-zinc-500">
              会追加到后端默认的亚马逊合规商品图模板中。
            </p>
          </label>

          <ProductImagePromptPlanner
            count={productImageCount}
            prompts={productImagePrompts}
            setPrompts={setProductImagePrompts}
          />
        </div>
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

function SDSImagePicker({
  availableImages,
  selectedImages,
  setSelectedImages,
}: {
  availableImages: SheinStudioSelectableSDSImage[];
  selectedImages: SheinStudioSelectedSDSImage[];
  setSelectedImages: (value: SheinStudioSelectedSDSImage[]) => void;
}) {
  const selectedMap = new Map(selectedImages.map((item) => [item.imageUrl, item]));

  function includeImage(image: SheinStudioSelectableSDSImage, asMain = false) {
    const next = selectedImages.filter((item) => item.imageUrl !== image.imageUrl);
    const payload = {
      imageUrl: image.imageUrl,
      variantSku: image.variantSku,
      color: image.color,
    } satisfies SheinStudioSelectedSDSImage;
    setSelectedImages(asMain ? [payload, ...next] : [...next, payload]);
  }

  function removeImage(imageUrl: string) {
    setSelectedImages(selectedImages.filter((item) => item.imageUrl !== imageUrl));
  }

  if (!availableImages.length) {
    return (
      <div className="rounded-2xl border border-dashed border-zinc-300 bg-white px-4 py-4 text-sm leading-6 text-zinc-600">
        当前选中的 SDS 商品还没有可用的官方渲染图。未手动选择时，系统会继续沿用后端自动匹配规则。
      </div>
    );
  }

  return (
    <div className="space-y-3 rounded-[1.5rem] border border-zinc-200 bg-white px-4 py-4">
      <SectionHeading
        eyebrow="SDS 图片"
        title="可选官方渲染图"
        description="可手动指定混合模式优先使用的 SDS 图片。已选第 1 张会作为 SDS 主图，其余按顺序进入 SDS 图库；不手动选择时，继续沿用系统自动匹配。"
      />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        {availableImages.map((image) => {
          const selected = selectedMap.get(image.imageUrl);
          const selectedIndex = selectedImages.findIndex(
            (item) => item.imageUrl === image.imageUrl,
          );
          return (
            <div
              key={image.imageUrl}
              className={`space-y-3 rounded-[1.25rem] border px-3 py-3 ${
                selected
                  ? "border-zinc-950 bg-zinc-950/[0.03]"
                  : "border-zinc-200 bg-zinc-50"
              }`}
            >
              <div className="relative aspect-square overflow-hidden rounded-2xl bg-zinc-100">
                <Image
                  alt={image.label}
                  className="object-cover"
                  fill
                  sizes="(max-width: 768px) 100vw, 240px"
                  src={image.imageUrl}
                />
              </div>
              <div className="space-y-1">
                <p className="text-sm font-semibold text-zinc-900">{image.label}</p>
                {image.description ? (
                  <p className="text-xs leading-5 text-zinc-600">{image.description}</p>
                ) : null}
                <p className="text-xs font-medium text-zinc-500">
                  {selectedIndex === 0
                    ? "当前主图"
                    : selectedIndex > 0
                      ? `已选第 ${selectedIndex + 1} 张`
                      : "未手动选择"}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button
                  className="h-9 px-3 text-xs"
                  onClick={() => includeImage(image, true)}
                  tone={selectedIndex === 0 ? "secondary" : "primary"}
                  type="button"
                >
                  {selectedIndex === 0 ? "已设为主图" : "设为主图"}
                </Button>
                <Button
                  className="h-9 px-3 text-xs"
                  onClick={() =>
                    selected ? removeImage(image.imageUrl) : includeImage(image, false)
                  }
                  tone="secondary"
                  type="button"
                >
                  {selected ? "移除" : "加入图库"}
                </Button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function ProductImagePromptPlanner({
  count,
  prompts,
  setPrompts,
}: {
  count: string;
  prompts: SheinStudioProductImagePrompt[];
  setPrompts: (value: SheinStudioProductImagePrompt[]) => void;
}) {
  const visibleCount = clampProductImageCount(count);
  const roles = SHEIN_STUDIO_PRODUCT_IMAGE_ROLES.slice(0, visibleCount);
  const promptByRole = new Map(prompts.map((item) => [item.role, item]));

  function updatePrompt(role: string, label: string, prompt: string) {
    setPrompts([
      ...prompts.filter((item) => item.role !== role),
      { label, prompt, role },
    ]);
  }

  return (
    <div className="space-y-3 rounded-[1.5rem] border border-zinc-200 bg-zinc-50 px-4 py-4">
      <div>
        <div className="text-sm font-semibold text-zinc-800">每张商品图提示词</div>
        <p className="mt-1 text-xs leading-5 text-zinc-500">
          可选。留空则使用该图片类型的默认模板。
        </p>
      </div>
      <div className="grid gap-3">
        {roles.map((role, index) => (
          <label
            className="rounded-2xl border border-zinc-200 bg-white px-3 py-3"
            key={role.role}
          >
            <span className="flex items-center justify-between gap-3 text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              <span>
                {index + 1}. {role.label}
              </span>
              <span className="text-[10px] text-zinc-400">{role.role}</span>
            </span>
            <textarea
              className="mt-2 min-h-20 w-full rounded-xl border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 focus:bg-white"
              onChange={(event) =>
                updatePrompt(role.role, role.label, event.target.value)
              }
              placeholder={role.hint}
              value={promptByRole.get(role.role)?.prompt ?? ""}
            />
          </label>
        ))}
      </div>
    </div>
  );
}

function SectionHeading({
  description,
  eyebrow,
  title,
}: {
  description: string;
  eyebrow: string;
  title: string;
}) {
  return (
    <div>
      <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-zinc-500">
        {eyebrow}
      </div>
      <h3 className="mt-1 text-lg font-semibold tracking-[-0.02em] text-zinc-950">
        {title}
      </h3>
      <p className="mt-1 text-xs leading-5 text-zinc-600">{description}</p>
    </div>
  );
}

function clampProductImageCount(value: string) {
  const parsed = Number.parseInt(value.trim(), 10);
  if (!Number.isFinite(parsed)) {
    return 1;
  }
  return Math.min(9, Math.max(1, parsed));
}

function NumberInput({
  label,
  max,
  min,
  setValue,
  value,
}: {
  label: string;
  max: number;
  min: number;
  setValue: (value: string) => void;
  value: string;
}) {
  return (
    <label className="space-y-2">
      <span className="text-sm font-medium text-zinc-700">{label}</span>
      <input
        className="w-full rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 focus:bg-white"
        inputMode="numeric"
        max={max}
        min={min}
        onChange={(event) => setValue(event.target.value)}
        value={value}
      />
    </label>
  );
}

function GenerationMessages({
  creatingError,
  creatingMessage,
  generationError,
  saveMessage,
  selectedStyleCount,
  selectionReady,
}: {
  creatingError: string;
  creatingMessage: string;
  generationError: string;
  saveMessage: string;
  selectedStyleCount: number;
  selectionReady: boolean;
}) {
  return (
    <>
      {!selectionReady ? (
        <Message tone="info">
          当前还不能生成或创建任务，请先回到第 1 步完成 SDS 商品选择。
        </Message>
      ) : null}
      {generationError ? (
        <Message tone="error">{generationError}</Message>
      ) : null}
      {creatingError ? <Message tone="error">{creatingError}</Message> : null}
      {creatingMessage ? <Message tone="info">{creatingMessage}</Message> : null}
      {selectedStyleCount > 0 ? (
        <Message tone="success">
          已选择 {selectedStyleCount} 个款式用于 SHEIN 审核。
        </Message>
      ) : null}
      {saveMessage ? <Message tone="neutral">{saveMessage}</Message> : null}
    </>
  );
}

function Message({
  children,
  tone,
}: {
  children: ReactNode;
  tone: "error" | "info" | "neutral" | "success";
}) {
  const classes = {
    error: "border-rose-200 bg-rose-50 text-rose-700",
    info: "border-sky-200 bg-sky-50 text-sky-800",
    neutral: "border-zinc-200 bg-zinc-50 text-zinc-600",
    success: "border-emerald-200 bg-emerald-50 text-emerald-800",
  };

  return (
    <div className={`rounded-2xl border px-4 py-3 text-sm ${classes[tone]}`}>
      {children}
    </div>
  );
}
