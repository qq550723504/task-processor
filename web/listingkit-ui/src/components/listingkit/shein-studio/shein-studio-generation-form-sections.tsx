import type { RefObject } from "react";
import { Upload } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import {
  NumberInput,
  ProductImagePromptPlanner,
  SDSImagePicker,
  SectionHeading,
} from "@/components/listingkit/shein-studio/shein-studio-generation-sections";
import { useSheinStoreSelector } from "@/lib/query/use-shein-store-selector";
import type { SheinStudioSelectableSDSImage } from "@/lib/shein-studio/sds-selectable-images";
import { formatSheinStoreOptionLabel } from "@/lib/shein-studio/store-option-label";
import type {
  SDSGroupedPromptHistoryEntry,
  SheinStudioArtworkModel,
  SheinStudioArtworkGenerationMode,
  SheinStudioGroupedImageMode,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";

export function ArtworkGenerationSettings({
  artworkModel,
  disabled,
  groupedImageMode,
  handleAnalyzeReferenceStyle,
  handleUploadHotStyleReferenceImages,
  artworkGenerationMode,
  hotStyleReferenceBrief,
  hotStyleReferenceImageUrls,
  hotStyleReferencePrompt,
  hotStyleReferenceWarnings,
  hotStyleReferenceUploadMessage,
  isAnalyzingReferenceStyle,
  isUploadingHotStyleReferenceImages,
  prompt,
  promptMode,
  promptHistory,
  promptInputRef,
  restorePrompt,
  setArtworkModel,
  setGroupedImageMode,
  setArtworkGenerationMode,
  setHotStyleReferenceImageUrls,
  setHotStyleReferencePrompt,
  selectedHotStyleReferenceFiles,
  setSelectedHotStyleReferenceFiles,
  setPrompt,
  setPromptMode,
  setStyleCount,
  setTransparentBackground,
  setVariationIntensity,
  showVariationIntensity,
  styleCount,
  transparentBackground,
  variationIntensity,
}: {
  artworkModel: SheinStudioArtworkModel;
  disabled?: boolean;
  groupedImageMode: SheinStudioGroupedImageMode;
  handleAnalyzeReferenceStyle: () => void;
  handleUploadHotStyleReferenceImages: () => void;
  artworkGenerationMode: SheinStudioArtworkGenerationMode;
  hotStyleReferenceBrief: string;
  hotStyleReferenceImageUrls: string[];
  hotStyleReferencePrompt: string;
  hotStyleReferenceWarnings?: string[];
  hotStyleReferenceUploadMessage?: string;
  isAnalyzingReferenceStyle: boolean;
  isUploadingHotStyleReferenceImages: boolean;
  prompt: string;
  promptMode: "managed" | "raw";
  promptHistory: SDSGroupedPromptHistoryEntry[];
  promptInputRef: RefObject<HTMLTextAreaElement | null>;
  restorePrompt: (value: string) => void;
  setArtworkModel: (value: SheinStudioArtworkModel) => void;
  setGroupedImageMode: (value: SheinStudioGroupedImageMode) => void;
  setArtworkGenerationMode: (value: SheinStudioArtworkGenerationMode) => void;
  setHotStyleReferenceImageUrls: (value: string[]) => void;
  setHotStyleReferencePrompt: (value: string) => void;
  selectedHotStyleReferenceFiles: File[];
  setSelectedHotStyleReferenceFiles: (value: File[]) => void;
  setPrompt: (value: string) => void;
  setPromptMode: (value: "managed" | "raw") => void;
  setStyleCount: (value: string) => void;
  setTransparentBackground: (value: boolean) => void;
  setVariationIntensity: (value: SheinStudioVariationIntensity) => void;
  showVariationIntensity: boolean;
  styleCount: string;
  transparentBackground: boolean;
  variationIntensity: SheinStudioVariationIntensity;
}) {
  const hotStyleReferenceAnalyzed = hotStyleReferenceBrief.trim().length > 0;
  const isHotReferenceMode = artworkGenerationMode === "hot_reference";

  return (
    <div className="space-y-4 rounded-[1.5rem] border border-emerald-200 bg-[linear-gradient(135deg,_#ecfdf5,_#f8fafc)] px-4 py-4 dark:border-emerald-500/25 dark:bg-emerald-950/15">
      <SectionHeading
        eyebrow="款式图"
        title="生成 POD 款式图"
        description="这里生成的是用于印刷的平面图案。商品场景图在下一块设置。"
      />
      <div className="grid grid-cols-2 gap-2 rounded-2xl border border-emerald-200 bg-background/80 p-1">
        <Button
          disabled={disabled}
          onClick={() => setArtworkGenerationMode("theme_prompt")}
          type="button"
          variant={
            artworkGenerationMode === "theme_prompt" ? "default" : "ghost"
          }
        >
          按主题生成
        </Button>
        <Button
          disabled={disabled}
          onClick={() => setArtworkGenerationMode("hot_reference")}
          type="button"
          variant={isHotReferenceMode ? "default" : "ghost"}
        >
          参考热销款
        </Button>
      </div>
      {!isHotReferenceMode ? (
        <Label className="space-y-2">
          <span className="text-sm font-medium text-foreground">
            主题提示词 <span className="text-rose-600">*</span>
          </span>
          <Textarea
            className="min-h-40 rounded-2xl border-emerald-200 bg-background/90 px-4 py-3 focus:border-emerald-900 focus:bg-background dark:border-emerald-500/25 dark:bg-background/80"
            disabled={disabled}
            onChange={(event) => setPrompt(event.target.value)}
            placeholder="例如：美国国旗主题，复古学院风，线条清晰，适合印刷。"
            ref={promptInputRef}
            value={prompt}
          />
          <p className="text-xs leading-6 text-muted-foreground">
            系统会优先生成适合 POD
            印刷的图案：大面积形状、清晰对比、减少细线和过小文字。
          </p>
          {promptHistory.length > 0 ? (
            <div className="rounded-2xl border border-emerald-200/80 bg-background/80 px-3 py-3 dark:border-emerald-500/20 dark:bg-card/90">
              <p className="text-xs font-semibold uppercase tracking-[0.2em] text-emerald-900/70">
                最近使用过的提示词
              </p>
              <div className="mt-2 flex flex-wrap gap-2">
                {promptHistory.map((entry) => (
                  <button
                    className="max-w-full rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-left text-xs text-emerald-950 transition hover:border-emerald-400 hover:bg-emerald-100 dark:border-emerald-500/25 dark:bg-emerald-950/30 dark:text-emerald-100 dark:hover:bg-emerald-950/45"
                    key={entry.createdAt}
                    onClick={() => restorePrompt(entry.prompt)}
                    type="button"
                  >
                    {entry.prompt}
                  </button>
                ))}
              </div>
            </div>
          ) : null}
        </Label>
      ) : null}
      {isHotReferenceMode ? (
        <div className="space-y-3 rounded-lg border border-border bg-background px-3 py-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <p className="text-sm font-medium text-foreground">热销款参考</p>
              <p className="text-xs leading-5 text-muted-foreground">
                提取图案、配色和构图方向，生成相似风格的原创 POD 图案。
              </p>
            </div>
            <Button
              disabled={
                disabled ||
                hotStyleReferenceImageUrls.length === 0 ||
                isAnalyzingReferenceStyle
              }
              onClick={handleAnalyzeReferenceStyle}
              type="button"
            >
              {isAnalyzingReferenceStyle ? "提取中..." : "提取热销款风格"}
            </Button>
          </div>
          <div className="grid gap-3 rounded-lg border border-dashed border-emerald-200 bg-emerald-50/60 px-3 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
            <Label className="space-y-2">
              <span className="text-sm font-medium text-foreground">
                上传热销参考图
              </span>
              <Input
                accept="image/*"
                aria-label="上传热销参考图"
                disabled={disabled || isUploadingHotStyleReferenceImages}
                onChange={(event) =>
                  setSelectedHotStyleReferenceFiles(
                    Array.from(event.target.files ?? []).slice(0, 1),
                  )
                }
                type="file"
              />
              <span className="block text-xs leading-5 text-muted-foreground">
                {selectedHotStyleReferenceFiles.length > 0
                  ? `已选择 ${selectedHotStyleReferenceFiles.length} 张图片`
                  : "可上传 1 张本地图片，也可以在下方粘贴 1 个公网 URL。"}
              </span>
            </Label>
            <Button
              disabled={
                disabled ||
                selectedHotStyleReferenceFiles.length === 0 ||
                isUploadingHotStyleReferenceImages
              }
              onClick={handleUploadHotStyleReferenceImages}
              type="button"
            >
              <Upload className="size-4" />
              {isUploadingHotStyleReferenceImages ? "上传中..." : "上传参考图"}
            </Button>
            {hotStyleReferenceUploadMessage ? (
              <p className="text-xs leading-5 text-muted-foreground sm:col-span-2">
                {hotStyleReferenceUploadMessage}
              </p>
            ) : null}
          </div>
          <Textarea
            className="min-h-20 rounded-lg px-3 py-2"
            disabled={disabled}
            onChange={(event) =>
              setHotStyleReferenceImageUrls(
                event.target.value
                  .split(/\r?\n/)
                  .map((value) => value.trim())
                  .filter(Boolean)
                  .slice(0, 1),
              )
            }
            placeholder="填写 1 个热销款参考图 URL。"
            value={(hotStyleReferenceImageUrls[0] ?? "").trim()}
          />
          <Textarea
            className="min-h-24 rounded-lg px-3 py-2"
            disabled={disabled || !hotStyleReferenceAnalyzed}
            onChange={(event) => setHotStyleReferencePrompt(event.target.value)}
            placeholder={
              hotStyleReferenceAnalyzed
                ? "提取后可在这里微调风格要求。"
                : "先提取热销款风格后再微调。"
            }
            value={hotStyleReferencePrompt}
          />
          {hotStyleReferenceBrief ? (
            <p className="text-xs leading-5 text-muted-foreground">
              {hotStyleReferenceBrief}
            </p>
          ) : null}
          {hotStyleReferenceWarnings && hotStyleReferenceWarnings.length > 0 ? (
            <div className="rounded-lg border border-amber-200 bg-amber-50/80 px-3 py-2 text-xs text-amber-900">
              <ul className="space-y-1">
                {hotStyleReferenceWarnings.map((warning, index) => (
                  <li key={`${index}-${warning}`}>{warning}</li>
                ))}
              </ul>
            </div>
          ) : null}
        </div>
      ) : null}
      {!isHotReferenceMode ? (
        <Label className="space-y-2">
          <span className="text-sm font-medium text-foreground">
            提示词模式
          </span>
          <Select
            disabled={disabled}
            onChange={(event) =>
              setPromptMode(event.target.value as "managed" | "raw")
            }
            value={promptMode}
          >
            <option value="managed">ListingKit 优化</option>
            <option value="raw">完全使用我的提示词</option>
          </Select>
        </Label>
      ) : null}
      <NumberInput
        disabled={disabled}
        label="款式数量"
        max={5}
        min={1}
        setValue={setStyleCount}
        value={styleCount}
      />
      <Label className="space-y-2">
        <span className="text-sm font-medium text-foreground">
          分组出图策略
        </span>
        <Select
          className="h-11 rounded-2xl border-emerald-200 bg-background/90 px-4 py-2 leading-5 focus:border-emerald-900 focus:bg-background dark:border-emerald-500/25 dark:bg-background/80"
          disabled={disabled}
          onChange={(event) =>
            setGroupedImageMode(
              event.target.value as SheinStudioGroupedImageMode,
            )
          }
          value={groupedImageMode}
        >
          <option value="shared_by_size">同尺寸共图（推荐）</option>
          <option value="per_product">每商品独立出图</option>
        </Select>
        <p className="text-xs leading-6 text-muted-foreground">
          同尺寸共图会按 printable size 自动复用款式图；每商品独立出图会为每个
          SDS 商品分别生成。
        </p>
      </Label>
      {showVariationIntensity ? (
        <Label className="space-y-2">
          <span className="text-sm font-medium text-foreground">变化强度</span>
          <Select
            className="h-11 rounded-2xl border-emerald-200 bg-background/90 px-4 py-2 leading-5 focus:border-emerald-900 focus:bg-background dark:border-emerald-500/25 dark:bg-background/80"
            disabled={disabled}
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
          </Select>
          <p className="text-xs leading-6 text-muted-foreground">
            只影响款式图批量生成。系统会保持同一核心卖点和视觉风格，同时按强度拉开构图和元素差异。
          </p>
        </Label>
      ) : null}
      <div className="grid gap-4 lg:grid-cols-2">
        <Label className="space-y-2">
          <span className="text-sm font-medium text-foreground">
            款式图模型
          </span>
          <Input
            className="rounded-2xl border-emerald-200 bg-background/90 px-4 py-3 focus:border-emerald-900 focus:bg-background dark:border-emerald-500/25 dark:bg-background/80"
            disabled={disabled}
            list="shein-studio-artwork-models"
            onChange={(event) => {
              const nextModel = event.target.value.trim();
              setArtworkModel(nextModel);
              if (nextModel !== "gpt-image-2" && transparentBackground) {
                setTransparentBackground(false);
              }
            }}
            placeholder="留空时跟随后端 image client 默认模型"
            value={transparentBackground ? "gpt-image-2" : artworkModel}
          />
          <datalist id="shein-studio-artwork-models">
            <option value="gpt-image-2" />
            <option value="nanobanana" />
            <option value="nano-banana-fast" />
            <option value="gpt-image-1" />
          </datalist>
          <p className="text-xs leading-6 text-muted-foreground">
            不再固定为前端两个选项。留空时使用设置页里 `image` client
            保存的默认模型，也可以直接手填覆盖。
          </p>
        </Label>
        <Label className="flex items-start gap-3 rounded-2xl border border-emerald-200 bg-background/80 px-4 py-3 dark:border-emerald-500/25 dark:bg-card/90">
          <Checkbox
            checked={transparentBackground}
            className="mt-1 border-emerald-300"
            disabled={disabled}
            onChange={(event) => {
              const checked = event.target.checked;
              setTransparentBackground(checked);
              if (checked) {
                setArtworkModel("gpt-image-2");
              }
            }}
          />
          <span className="text-sm leading-6 text-emerald-950">
            <span className="block font-semibold">生成透明背景图案</span>
            <span className="block text-xs text-emerald-800">
              开启后仍会强制切换到 `gpt-image-2`，并要求输出真正 alpha
              透明背景。
            </span>
          </span>
        </Label>
      </div>
    </div>
  );
}

export function ProductImageGenerationSettings({
  availableSdsImages,
  imageStrategy,
  productImageCount,
  productImagePrompt,
  productImagePrompts,
  renderSizeImagesWithSds,
  selectedSdsImages,
  setImageStrategy,
  setProductImageCount,
  setProductImagePrompt,
  setProductImagePrompts,
  setRenderSizeImagesWithSds,
  setSelectedSdsImages,
  showRenderSizeImagesWithSdsOption,
}: {
  availableSdsImages: SheinStudioSelectableSDSImage[];
  imageStrategy: SheinStudioImageStrategy;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  renderSizeImagesWithSds: boolean;
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  setImageStrategy: (value: SheinStudioImageStrategy) => void;
  setProductImageCount: (value: string) => void;
  setProductImagePrompt: (value: string) => void;
  setProductImagePrompts: (value: SheinStudioProductImagePrompt[]) => void;
  setRenderSizeImagesWithSds: (value: boolean) => void;
  setSelectedSdsImages: (value: SheinStudioSelectedSDSImage[]) => void;
  showRenderSizeImagesWithSdsOption: boolean;
}) {
  return (
    <div className="space-y-4 rounded-[1.5rem] border border-border bg-muted px-4 py-4">
      <SectionHeading
        eyebrow="商品图"
        title="设置上架商品图生成"
        description="审核通过的款式转成 SHEIN 资料时，会使用这里的商品图设置。"
      />
      {imageStrategy !== "sds_official" ? (
        <div className="grid gap-4 lg:grid-cols-2">
          <NumberInput
            label="商品图数量"
            max={9}
            min={1}
            setValue={setProductImageCount}
            value={productImageCount}
          />
        </div>
      ) : null}

      <Label className="space-y-2">
        <span className="text-sm font-medium text-foreground">图片策略</span>
        <Select
          className="h-11 rounded-2xl px-4 py-2 leading-5"
          onChange={(event) =>
            setImageStrategy(event.target.value as SheinStudioImageStrategy)
          }
          value={imageStrategy}
        >
          <option value="ai_generated">AI 生成商品图</option>
          <option value="sds_official">SDS 官方渲染</option>
          <option value="hybrid">混合：SDS 主图 + AI 图库</option>
        </Select>
        <p className="text-xs leading-6 text-muted-foreground">
          AI 生成模式不调用 SDS 设计器；SDS 官方渲染会使用模板图； 混合模式先用
          SDS 图，再追加 AI 商品图。
        </p>
      </Label>

      {imageStrategy === "hybrid" || imageStrategy === "sds_official" ? (
        <SDSImagePicker
          availableImages={availableSdsImages}
          selectedImages={selectedSdsImages}
          setSelectedImages={setSelectedSdsImages}
        />
      ) : null}

      {showRenderSizeImagesWithSdsOption ? (
        <Label className="flex items-start gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3">
          <Checkbox
            checked={renderSizeImagesWithSds}
            className="mt-1 border-amber-300"
            onChange={(event) =>
              setRenderSizeImagesWithSds(event.target.checked)
            }
          />
          <span className="text-sm leading-6 text-amber-950">
            <span className="block font-semibold">尺寸图也使用 SDS 渲染</span>
            <span className="block text-xs text-amber-800">
              AI 或混合模式下会额外调用 SDS，只取尺寸图用于 SHEIN
              尺寸图，不替换主图和场景图。
            </span>
          </span>
        </Label>
      ) : null}

      {imageStrategy === "ai_generated" ? (
        <>
          <Label className="space-y-2">
            <span className="text-sm font-medium text-foreground">
              全局商品图提示词
            </span>
            <Textarea
              className="min-h-24 rounded-2xl px-4 py-3"
              onChange={(event) => setProductImagePrompt(event.target.value)}
              placeholder="可选。会应用到每一张商品图，例如：背景保持暖色、简洁。"
              value={productImagePrompt}
            />
            <p className="text-xs leading-6 text-muted-foreground">
              会追加到后端默认的亚马逊合规商品图模板中。
            </p>
          </Label>

          <ProductImagePromptPlanner
            count={productImageCount}
            prompts={productImagePrompts}
            setPrompts={setProductImagePrompts}
          />
        </>
      ) : null}
    </div>
  );
}

export function BatchStoreSettings({
  currentStoreLabel,
  requiredMessage,
  sheinStoreId,
  setSheinStoreId,
}: {
  currentStoreLabel?: string;
  requiredMessage?: string;
  sheinStoreId: string;
  setSheinStoreId: (value: string) => void;
}) {
  const { enabledProfiles, profiles } = useSheinStoreSelector();

  return (
    <div
      className={`rounded-2xl px-4 py-4 ${
        requiredMessage
          ? "border border-rose-200 bg-rose-50/80"
          : "border border-border bg-muted/80"
      }`}
    >
      <div className="space-y-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
            批次店铺
          </p>
          <p className="mt-1 text-sm text-muted-foreground">
            批次内商品默认跟随这里，生成和建任务前也需要先确定它。
          </p>
        </div>
        {requiredMessage ? (
          <div className="rounded-2xl border border-rose-200 bg-background px-4 py-3 text-sm leading-6 text-rose-700 dark:border-rose-500/30 dark:bg-card">
            请先设置批次店铺，否则当前不能生成或创建 SHEIN 资料。
          </div>
        ) : currentStoreLabel ? (
          <div className="rounded-2xl border border-emerald-200 bg-background px-4 py-3 dark:border-emerald-500/25 dark:bg-card">
            <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-emerald-700">
              当前默认跟随
            </div>
            <div className="mt-1 text-sm font-semibold leading-6 text-emerald-800">
              {currentStoreLabel}
            </div>
          </div>
        ) : null}
        <Label className="space-y-2">
          <span className="text-sm font-medium text-foreground">
            选择批次店铺
          </span>
          <Select
            aria-label="批次店铺"
            className="h-11 rounded-2xl px-4 py-2 leading-5"
            onChange={(event) => setSheinStoreId(event.target.value)}
            value={sheinStoreId}
          >
            <option value="">
              {enabledProfiles.length > 0
                ? "请选择批次店铺"
                : "当前没有已启用店铺配置"}
            </option>
            {enabledProfiles.map((item) => (
              <option
                key={item.id ?? item.store_id}
                value={String(item.store_id)}
              >
                {formatSheinStoreOptionLabel(item)}
              </option>
            ))}
          </Select>
          {profiles.isError ? (
            <p className="text-xs leading-6 text-rose-600">
              店铺配置读取失败，请先检查批次店铺配置。
            </p>
          ) : null}
        </Label>
      </div>
    </div>
  );
}
