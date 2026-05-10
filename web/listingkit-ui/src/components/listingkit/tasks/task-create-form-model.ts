import { z } from "zod";

import type { TaskCreateDraft } from "@/components/listingkit/tasks/task-create-draft";
import type { TaskSourceTab } from "@/components/listingkit/tasks/task-source-tabs";
export const platformOptions = [
  { value: "amazon", label: "Amazon" },
  { value: "shein", label: "SHEIN" },
  { value: "temu", label: "Temu" },
  { value: "walmart", label: "Walmart" },
] as const;

export const sceneCategoryOptions = [
  { value: "", label: "自动" },
  { value: "shoes", label: "鞋履" },
  { value: "jewelry", label: "饰品" },
  { value: "bags", label: "箱包" },
] as const;

export const sceneStyleOptions = [
  { value: "", label: "自动" },
  { value: "studio", label: "棚拍" },
  { value: "lifestyle", label: "生活方式" },
  { value: "outdoor", label: "户外" },
  { value: "minimal", label: "极简" },
] as const;

export const backgroundToneOptions = [
  { value: "", label: "自动" },
  { value: "warm", label: "暖色" },
  { value: "cool", label: "冷色" },
  { value: "neutral", label: "中性" },
  { value: "bright", label: "明亮" },
] as const;

export const compositionOptions = [
  { value: "", label: "自动" },
  { value: "centered", label: "居中" },
  { value: "close_up", label: "特写" },
  { value: "multi_angle", label: "多角度" },
] as const;

export const propsLevelOptions = [
  { value: "", label: "自动" },
  { value: "none", label: "无" },
  { value: "light", label: "轻量" },
  { value: "moderate", label: "适中" },
] as const;

export const audienceHintOptions = [
  { value: "", label: "自动" },
  { value: "premium", label: "高端" },
  { value: "youthful", label: "年轻化" },
  { value: "sporty", label: "运动感" },
  { value: "homey", label: "居家感" },
] as const;

export const schema = z
  .object({
    text: z.string().trim(),
    imageUrls: z.string().trim(),
    productUrl: z.string().trim(),
    platforms: z.array(z.string()).min(1, "请至少选择一个平台。"),
    sheinStoreId: z.string().trim(),
    sdsEnabled: z.boolean(),
    sdsVariantId: z.string().trim(),
    sdsParentProductId: z.string().trim(),
    sdsPrototypeGroupId: z.string().trim(),
    sdsLayerId: z.string().trim(),
    sdsDesignType: z.string().trim(),
    sdsFitLevel: z.string().trim(),
    sdsResizeMode: z.string().trim(),
    sceneCategory: z.string().trim(),
    sceneStyle: z.string().trim(),
    backgroundTone: z.string().trim(),
    composition: z.string().trim(),
    propsLevel: z.string().trim(),
    audienceHint: z.string().trim(),
    customSceneHint: z.string().trim(),
  });

export type FormValues = z.infer<typeof schema>;

export function parseImageUrls(input: string) {
  return input
    .split(/\r?\n/)
    .map((value) => value.trim())
    .filter(Boolean);
}

export function parseOptionalPositiveInt(input: string) {
  const trimmed = input.trim();
  if (!trimmed) {
    return undefined;
  }
  const parsed = Number.parseInt(trimmed, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return undefined;
  }
  return parsed;
}

function parseOptionalInt(input: string) {
  const trimmed = input.trim();
  if (!trimmed) {
    return undefined;
  }
  const parsed = Number.parseInt(trimmed, 10);
  if (!Number.isFinite(parsed)) {
    return undefined;
  }
  return parsed;
}

function parseOptionalNumber(input: string) {
  const trimmed = input.trim();
  if (!trimmed) {
    return undefined;
  }
  const parsed = Number(trimmed);
  if (!Number.isFinite(parsed)) {
    return undefined;
  }
  return parsed;
}

export function parseSelectedVariantIds(input: string | null) {
  if (!input?.trim()) {
    return undefined;
  }
  const ids = input
    .split(",")
    .map((item) => Number.parseInt(item.trim(), 10))
    .filter((item) => Number.isFinite(item) && item > 0);
  return ids.length > 0 ? ids : undefined;
}

export function buildSDSOptions(values: FormValues) {
  if (!values.sdsEnabled) {
    return undefined;
  }

  const variantId = parseOptionalPositiveInt(values.sdsVariantId ?? "");
  if (!variantId) {
    return undefined;
  }

  return {
    variant_id: variantId,
    ...(parseOptionalPositiveInt(values.sdsParentProductId ?? "")
      ? { parent_product_id: parseOptionalPositiveInt(values.sdsParentProductId ?? "") }
      : {}),
    ...(parseOptionalPositiveInt(values.sdsPrototypeGroupId ?? "")
      ? { prototype_group_id: parseOptionalPositiveInt(values.sdsPrototypeGroupId ?? "") }
      : {}),
    ...(values.sdsLayerId.trim() ? { layer_id: values.sdsLayerId.trim() } : {}),
    ...(values.sdsDesignType.trim() ? { design_type: values.sdsDesignType.trim() } : {}),
    ...(parseOptionalNumber(values.sdsFitLevel ?? "") !== undefined
      ? { fit_level: parseOptionalNumber(values.sdsFitLevel ?? "") }
      : {}),
    ...(parseOptionalInt(values.sdsResizeMode ?? "") !== undefined
      ? { resize_mode: parseOptionalInt(values.sdsResizeMode ?? "") }
      : {}),
  };
}

export function buildSceneOptions(values: FormValues) {
  const scene = {
    scene_category: values.sceneCategory,
    scene_style: values.sceneStyle,
    background_tone: values.backgroundTone,
    composition: values.composition,
    props_level: values.propsLevel,
    audience_hint: values.audienceHint,
    custom_scene_hint: values.customSceneHint,
  };

  return Object.values(scene).some((value) => value.trim())
    ? scene
    : undefined;
}

export function inferInitialSourceTab({
  initialValues,
  initialFocus,
}: {
  initialValues?: Partial<TaskCreateDraft>;
  initialFocus?: "text" | "imageUrls" | "productUrl";
}): TaskSourceTab {
  if (initialFocus === "productUrl") {
    return "productUrl";
  }
  if (initialFocus === "imageUrls") {
    return "imageUrls";
  }
  if (initialValues?.productUrl?.trim()) {
    return "productUrl";
  }

  return "imageUrls";
}

export function titleFieldCopy(activeSourceTab: TaskSourceTab) {
  if (activeSourceTab === "productUrl") {
    return {
      label: "选填标题",
      helper: "如果已经提供商品链接，这里不是必填；只有想覆盖原始标题时再填写。",
    };
  }

  return {
    label: "商品标题",
    helper: "适合从图片开始创建任务。标题越完整，生成质量通常越稳定。",
  };
}


