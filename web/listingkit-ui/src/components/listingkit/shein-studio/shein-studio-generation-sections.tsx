import Image from "next/image";
import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import type { SheinStudioSelectableSDSImage } from "@/lib/shein-studio/sds-selectable-images";
import { SHEIN_STUDIO_PRODUCT_IMAGE_ROLES } from "@/lib/shein-studio/storage-shared";
import type {
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
} from "@/lib/types/shein-studio";

export function SectionHeading({
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

export function clampProductImageCount(value: string) {
  const parsed = parsePositiveInteger(value);
  if (!Number.isFinite(parsed)) {
    return 1;
  }
  return Math.min(9, Math.max(1, parsed));
}

export function parsePositiveInteger(value: string) {
  return Number.parseInt(value.trim(), 10);
}

export function NumberInput({
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
    <Label className="space-y-2">
      <span className="text-sm font-medium text-zinc-700">{label}</span>
      <Input
        className="rounded-2xl bg-zinc-50 px-4 py-3 focus:bg-white"
        inputMode="numeric"
        max={max}
        min={min}
        onChange={(event) => setValue(event.target.value)}
        value={value}
      />
    </Label>
  );
}

export function GenerationMessages({
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

export function Message({
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

export function SDSImagePicker({
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
                  variant={selectedIndex === 0 ? "secondary" : "primary"}
                  type="button"
                >
                  {selectedIndex === 0 ? "已设为主图" : "设为主图"}
                </Button>
                <Button
                  className="h-9 px-3 text-xs"
                  onClick={() =>
                    selected ? removeImage(image.imageUrl) : includeImage(image, false)
                  }
                  variant="secondary"
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

export function ProductImagePromptPlanner({
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
          <Label
            className="rounded-2xl border border-zinc-200 bg-white px-3 py-3"
            key={role.role}
          >
            <span className="flex items-center justify-between gap-3 text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              <span>
                {index + 1}. {role.label}
              </span>
              <span className="text-[10px] text-zinc-400">{role.role}</span>
            </span>
            <Textarea
              className="mt-2 min-h-20 rounded-xl bg-zinc-50 focus:bg-white"
              onChange={(event) =>
                updatePrompt(role.role, role.label, event.target.value)
              }
              placeholder={role.hint}
              value={promptByRole.get(role.role)?.prompt ?? ""}
            />
          </Label>
        ))}
      </div>
    </div>
  );
}
