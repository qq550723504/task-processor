import type { RefObject } from "react";

import {
  NumberInput,
  ProductImagePromptPlanner,
  SDSImagePicker,
  SectionHeading,
} from "@/components/listingkit/shein-studio/shein-studio-generation-sections";
import { DEFAULT_SHEIN_STORE_ID } from "@/lib/shein-studio/create-review-tasks";
import type { SheinStudioSelectableSDSImage } from "@/lib/shein-studio/sds-selectable-images";
import type {
  SheinStudioArtworkModel,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";

export function ArtworkGenerationSettings({
  artworkModel,
  prompt,
  promptInputRef,
  setArtworkModel,
  setPrompt,
  setStyleCount,
  setTransparentBackground,
  setVariationIntensity,
  showVariationIntensity,
  styleCount,
  transparentBackground,
  variationIntensity,
}: {
  artworkModel: SheinStudioArtworkModel;
  prompt: string;
  promptInputRef: RefObject<HTMLTextAreaElement | null>;
  setArtworkModel: (value: SheinStudioArtworkModel) => void;
  setPrompt: (value: string) => void;
  setStyleCount: (value: string) => void;
  setTransparentBackground: (value: boolean) => void;
  setVariationIntensity: (value: SheinStudioVariationIntensity) => void;
  showVariationIntensity: boolean;
  styleCount: string;
  transparentBackground: boolean;
  variationIntensity: SheinStudioVariationIntensity;
}) {
  return (
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
      {showVariationIntensity ? (
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
      ) : null}
      <div className="grid gap-4 lg:grid-cols-2">
        <label className="space-y-2">
          <span className="text-sm font-medium text-zinc-700">款式图模型</span>
          <input
            className="w-full rounded-2xl border border-emerald-200 bg-white/80 px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-emerald-900 focus:bg-white"
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
          <p className="text-xs leading-6 text-zinc-600">
            不再固定为前端两个选项。留空时使用设置页里 `image` client 保存的默认模型，也可以直接手填覆盖。
          </p>
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
              开启后仍会强制切换到 `gpt-image-2`，并要求输出真正 alpha
              透明背景。
            </span>
          </span>
        </label>
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
  setSheinStoreId,
  sheinStoreId,
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
  setSheinStoreId: (value: string) => void;
  sheinStoreId: string;
  showRenderSizeImagesWithSdsOption: boolean;
}) {
  return (
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
          <span className="text-sm font-medium text-zinc-700">
            SHEIN 店铺 ID
          </span>
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
            onChange={(event) =>
              setRenderSizeImagesWithSds(event.target.checked)
            }
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
  );
}
