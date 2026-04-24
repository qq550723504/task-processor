"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useForm, useWatch } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";

import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import {
  saveTaskCreateDraft,
  type TaskCreateDraft,
} from "@/components/listingkit/task-create-draft";
import { TaskInputGuidance } from "@/components/listingkit/task-input-guidance";
import { TaskSDSOptions } from "@/components/listingkit/task-sds-options";
import {
  getPlatformSceneDefaults,
  hasAnySceneCustomization,
  matchesSceneDefaults,
} from "@/components/listingkit/task-scene-defaults";
import {
  TaskSourceTabs,
  type TaskSourceTab,
} from "@/components/listingkit/task-source-tabs";
import { useCreateTask } from "@/lib/query/use-create-task";
import { useUploadImages } from "@/lib/query/use-upload-images";

const platformOptions = [
  { value: "amazon", label: "Amazon" },
  { value: "shein", label: "Shein" },
  { value: "temu", label: "Temu" },
  { value: "walmart", label: "Walmart" },
] as const;

const sceneCategoryOptions = [
  { value: "", label: "Auto" },
  { value: "shoes", label: "Shoes" },
  { value: "jewelry", label: "Jewelry" },
  { value: "bags", label: "Bags" },
] as const;

const sceneStyleOptions = [
  { value: "", label: "Auto" },
  { value: "studio", label: "Studio" },
  { value: "lifestyle", label: "Lifestyle" },
  { value: "outdoor", label: "Outdoor" },
  { value: "minimal", label: "Minimal" },
] as const;

const backgroundToneOptions = [
  { value: "", label: "Auto" },
  { value: "warm", label: "Warm" },
  { value: "cool", label: "Cool" },
  { value: "neutral", label: "Neutral" },
  { value: "bright", label: "Bright" },
] as const;

const compositionOptions = [
  { value: "", label: "Auto" },
  { value: "centered", label: "Centered" },
  { value: "close_up", label: "Close up" },
  { value: "multi_angle", label: "Multi-angle" },
] as const;

const propsLevelOptions = [
  { value: "", label: "Auto" },
  { value: "none", label: "None" },
  { value: "light", label: "Light" },
  { value: "moderate", label: "Moderate" },
] as const;

const audienceHintOptions = [
  { value: "", label: "Auto" },
  { value: "premium", label: "Premium" },
  { value: "youthful", label: "Youthful" },
  { value: "sporty", label: "Sporty" },
  { value: "homey", label: "Homey" },
] as const;

const schema = z
  .object({
    text: z.string().trim(),
    imageUrls: z.string().trim(),
    productUrl: z.string().trim(),
    platforms: z.array(z.string()).min(1, "Select at least one platform."),
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

type FormValues = z.infer<typeof schema>;

function parseImageUrls(input: string) {
  return input
    .split(/\r?\n/)
    .map((value) => value.trim())
    .filter(Boolean);
}

function parseOptionalPositiveInt(input: string) {
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

function buildSDSOptions(values: FormValues) {
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

function buildSceneOptions(values: FormValues) {
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

function inferInitialSourceTab({
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

function titleFieldCopy(activeSourceTab: TaskSourceTab) {
  if (activeSourceTab === "productUrl") {
    return {
      label: "Optional title",
      helper:
        "Not required when you provide a product URL. Add it only if you want to override or improve the listing title.",
    };
  }

  return {
    label: "Product title",
    helper:
      "Recommended for image-driven creation. Stronger title text helps ListingKit pass the current quality gate.",
  };
}

export function TaskCreateForm({
  initialValues,
  initialFocus,
  fieldIssues,
  variant = "default",
}: {
  initialValues?: Partial<TaskCreateDraft>;
  initialFocus?: "text" | "imageUrls" | "productUrl";
  fieldIssues?: Array<"text" | "imageUrls" | "productUrl">;
  variant?: "default" | "sds";
}) {
  const router = useRouter();
  const createTask = useCreateTask();
  const uploadImages = useUploadImages();
  const textRef = useRef<HTMLInputElement | null>(null);
  const imageUrlsRef = useRef<HTMLTextAreaElement | null>(null);
  const productUrlRef = useRef<HTMLInputElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const lastAppliedSceneDefaultsRef = useRef<ReturnType<typeof getPlatformSceneDefaults>>(null);
  const [activeSourceTab, setActiveSourceTab] = useState<TaskSourceTab>(() =>
    inferInitialSourceTab({ initialValues, initialFocus }),
  );
  const [showSceneCustomization, setShowSceneCustomization] = useState(() =>
    Boolean(
      initialValues?.sceneCategory ||
        initialValues?.sceneStyle ||
        initialValues?.backgroundTone ||
        initialValues?.composition ||
        initialValues?.propsLevel ||
        initialValues?.audienceHint ||
        initialValues?.customSceneHint,
    ),
  );
  const [sdsEnabled, setSDSEnabled] = useState(() =>
    variant === "sds" || Boolean(initialValues?.sdsEnabled),
  );
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const {
    register,
    handleSubmit,
    setError,
    clearErrors,
    setValue,
    formState: { errors },
    control,
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      text: initialValues?.text ?? "",
      imageUrls: initialValues?.imageUrls ?? "",
      productUrl: initialValues?.productUrl ?? "",
      platforms:
        initialValues?.platforms && initialValues.platforms.length > 0
          ? initialValues.platforms
          : variant === "sds"
            ? ["amazon"]
            : [],
      sheinStoreId: initialValues?.sheinStoreId ?? "",
      sdsEnabled: variant === "sds" || Boolean(initialValues?.sdsEnabled),
      sdsVariantId: initialValues?.sdsVariantId ?? "",
      sdsParentProductId: initialValues?.sdsParentProductId ?? "",
      sdsPrototypeGroupId: initialValues?.sdsPrototypeGroupId ?? "",
      sdsLayerId: initialValues?.sdsLayerId ?? "",
      sdsDesignType: initialValues?.sdsDesignType ?? "material",
      sdsFitLevel: initialValues?.sdsFitLevel ?? "1",
      sdsResizeMode: initialValues?.sdsResizeMode ?? "0",
      sceneCategory: initialValues?.sceneCategory ?? "",
      sceneStyle: initialValues?.sceneStyle ?? "",
      backgroundTone: initialValues?.backgroundTone ?? "",
      composition: initialValues?.composition ?? "",
      propsLevel: initialValues?.propsLevel ?? "",
      audienceHint: initialValues?.audienceHint ?? "",
      customSceneHint: initialValues?.customSceneHint ?? "",
    },
  });

  const selectedPlatforms = useWatch({
    control,
    name: "platforms",
  });
  const currentText = useWatch({
    control,
    name: "text",
  });
  const currentImageUrls = useWatch({
    control,
    name: "imageUrls",
  });
  const currentProductUrl = useWatch({
    control,
    name: "productUrl",
  });
  const currentSheinStoreId = useWatch({
    control,
    name: "sheinStoreId",
  });
  const currentSDSEnabled = useWatch({
    control,
    name: "sdsEnabled",
  });
  const currentSDSVariantId = useWatch({
    control,
    name: "sdsVariantId",
  });
  const currentSDSParentProductId = useWatch({
    control,
    name: "sdsParentProductId",
  });
  const currentSDSPrototypeGroupId = useWatch({
    control,
    name: "sdsPrototypeGroupId",
  });
  const currentSDSLayerId = useWatch({
    control,
    name: "sdsLayerId",
  });
  const currentSDSDesignType = useWatch({
    control,
    name: "sdsDesignType",
  });
  const currentSDSFitLevel = useWatch({
    control,
    name: "sdsFitLevel",
  });
  const currentSDSResizeMode = useWatch({
    control,
    name: "sdsResizeMode",
  });
  const currentSceneCategory = useWatch({
    control,
    name: "sceneCategory",
  });
  const currentSceneStyle = useWatch({
    control,
    name: "sceneStyle",
  });
  const currentBackgroundTone = useWatch({
    control,
    name: "backgroundTone",
  });
  const currentComposition = useWatch({
    control,
    name: "composition",
  });
  const currentPropsLevel = useWatch({
    control,
    name: "propsLevel",
  });
  const currentAudienceHint = useWatch({
    control,
    name: "audienceHint",
  });
  const currentCustomSceneHint = useWatch({
    control,
    name: "customSceneHint",
  });
  const helperText = useMemo(
    () =>
      "Use public image URLs, upload local image files, or paste a product URL such as a 1688 listing.",
    [],
  );
  const imageCount = useMemo(
    () => parseImageUrls(currentImageUrls ?? "").length,
    [currentImageUrls],
  );
  const textLength = (currentText ?? "").trim().length;
  const textRegistration = register("text");
  const imageUrlsRegistration = register("imageUrls");
  const productUrlRegistration = register("productUrl");
  const titleCopy = titleFieldCopy(activeSourceTab);
  const pageCopy =
    variant === "sds"
      ? {
          eyebrow: "SDS Sync",
          title: "Create ListingKit task with SDS design sync",
          description:
            "Use the normal ListingKit pipeline, then automatically push the selected image asset into SDS after product image processing succeeds.",
          submitLabel: "Create task and sync SDS",
        }
      : {
          eyebrow: "ListingKit UI",
          title: "Create ListingKit task",
          description:
            "Start with a product title, image URLs, or a product URL such as a 1688 listing, then choose the target platforms.",
          submitLabel: "Create task",
        };
  const primaryPlatform = selectedPlatforms?.[0];
  const platformSceneDefaults = useMemo(
    () => getPlatformSceneDefaults(primaryPlatform, currentSceneCategory),
    [primaryPlatform, currentSceneCategory],
  );
  const sceneSummary = useMemo(() => {
    if (!primaryPlatform || !platformSceneDefaults) {
      return null;
    }
    const parts = [
      platformSceneDefaults.sceneStyle,
      platformSceneDefaults.backgroundTone,
      platformSceneDefaults.composition,
    ].filter(Boolean);
    return `${primaryPlatform} defaults: ${parts.join(" / ")}`;
  }, [platformSceneDefaults, primaryPlatform]);

  useEffect(() => {
    if (!showSceneCustomization || !platformSceneDefaults) {
      return;
    }
    const currentSceneValues = {
      sceneCategory: currentSceneCategory,
      sceneStyle: currentSceneStyle,
      backgroundTone: currentBackgroundTone,
      composition: currentComposition,
      propsLevel: currentPropsLevel,
      audienceHint: currentAudienceHint,
      customSceneHint: currentCustomSceneHint,
    };
    const canApplyDefaults =
      !hasAnySceneCustomization(currentSceneValues) ||
      matchesSceneDefaults(currentSceneValues, lastAppliedSceneDefaultsRef.current);
    if (!canApplyDefaults) {
      return;
    }
    setValue("sceneStyle", platformSceneDefaults.sceneStyle ?? "", {
      shouldDirty: true,
    });
    setValue("backgroundTone", platformSceneDefaults.backgroundTone ?? "", {
      shouldDirty: true,
    });
    setValue("composition", platformSceneDefaults.composition ?? "", {
      shouldDirty: true,
    });
    setValue("propsLevel", platformSceneDefaults.propsLevel ?? "", {
      shouldDirty: true,
    });
    setValue("audienceHint", platformSceneDefaults.audienceHint ?? "", {
      shouldDirty: true,
    });
    lastAppliedSceneDefaultsRef.current = platformSceneDefaults;
  }, [
    currentAudienceHint,
    currentBackgroundTone,
    currentComposition,
    currentCustomSceneHint,
    currentPropsLevel,
    currentSceneCategory,
    currentSceneStyle,
    platformSceneDefaults,
    setValue,
    showSceneCustomization,
  ]);

  useEffect(() => {
    if (initialFocus === "text") {
      textRef.current?.focus();
    }
  }, [initialFocus]);

  useEffect(() => {
    if (activeSourceTab === "productUrl") {
      productUrlRef.current?.focus();
      return;
    }
    imageUrlsRef.current?.focus();
  }, [activeSourceTab]);

  return (
    <Card
      className={
        variant === "sds"
          ? "mx-auto max-w-7xl rounded-[2rem] border-white/70 bg-white/78 p-8 shadow-[0_20px_80px_rgba(24,24,27,0.08)] backdrop-blur"
          : "mx-auto max-w-3xl p-8"
      }
    >
      <form
        className="space-y-6"
        onSubmit={handleSubmit(async (values) => {
          const text = (values.text ?? "").trim();
          const imageUrls = values.imageUrls ?? "";
          const productUrl = (values.productUrl ?? "").trim();
          const parsedImageUrls = parseImageUrls(imageUrls);
          if (!text && parsedImageUrls.length === 0 && !productUrl) {
            setError("root", {
              message:
                "Provide at least one of product title, image URLs, or product URL.",
            });
            return;
          }
          clearErrors("root");
          const draft = {
            text,
            imageUrls,
            productUrl,
            platforms: values.platforms,
            sheinStoreId: currentSheinStoreId,
            sdsEnabled: currentSDSEnabled,
            sdsVariantId: currentSDSVariantId,
            sdsParentProductId: currentSDSParentProductId,
            sdsPrototypeGroupId: currentSDSPrototypeGroupId,
            sdsLayerId: currentSDSLayerId,
            sdsDesignType: currentSDSDesignType,
            sdsFitLevel: currentSDSFitLevel,
            sdsResizeMode: currentSDSResizeMode,
            sceneCategory: values.sceneCategory,
            sceneStyle: values.sceneStyle,
            backgroundTone: values.backgroundTone,
            composition: values.composition,
            propsLevel: values.propsLevel,
            audienceHint: values.audienceHint,
            customSceneHint: values.customSceneHint,
          } satisfies TaskCreateDraft;
          const sceneOptions = buildSceneOptions(values);
          const sdsOptions = buildSDSOptions(values);
          if (values.sdsEnabled && !sdsOptions) {
            setError("root", {
              message: "SDS sync requires a valid positive Variant ID.",
            });
            return;
          }
          const sheinStoreId = parseOptionalPositiveInt(values.sheinStoreId ?? "");
          const options = {
            ...(sceneOptions || sdsOptions ? { process_images: true } : {}),
            ...(sceneOptions ? { scene: sceneOptions } : {}),
            ...(sdsOptions ? { sds: sdsOptions } : {}),
          };
          const request = {
            text: draft.text,
            image_urls: parsedImageUrls,
            platforms: values.platforms,
            ...(sheinStoreId ? { shein_store_id: sheinStoreId } : {}),
            ...(draft.productUrl ? { product_url: draft.productUrl } : {}),
            ...(Object.keys(options).length > 0 ? { options } : {}),
          };
          const task = await createTask.mutateAsync(request);
          saveTaskCreateDraft(task.task_id, draft);
          router.push(`/listing-kits/${task.task_id}/status`);
        })}
      >
        <div className="space-y-2">
          <p
            className={
              variant === "sds"
                ? "text-[11px] font-semibold uppercase tracking-[0.3em] text-emerald-700"
                : "text-xs font-semibold uppercase tracking-[0.24em] text-zinc-500"
            }
          >
            {pageCopy.eyebrow}
          </p>
          <h1
            className={
              variant === "sds"
                ? "font-serif text-4xl tracking-[-0.04em] text-zinc-950"
                : "text-3xl font-semibold tracking-tight text-zinc-950"
            }
          >
            {pageCopy.title}
          </h1>
          <p className="text-sm leading-6 text-zinc-600">
            {pageCopy.description}
          </p>
        </div>

        <TaskSourceTabs
          activeTab={activeSourceTab}
          onTabChange={(tab) => {
            setActiveSourceTab(tab);
          }}
        />

        <label className="block space-y-2">
          <span className="text-sm font-medium text-zinc-700">{titleCopy.label}</span>
          <input
            aria-label="Product title"
            className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
            placeholder="Women knit cardigan"
            {...textRegistration}
            ref={(element) => {
              textRef.current = element;
              textRegistration.ref(element);
            }}
          />
          <p className="text-sm leading-6 text-zinc-500">{titleCopy.helper}</p>
          {errors.text ? (
            <p className="text-sm text-red-600">{errors.text.message}</p>
          ) : null}
          {fieldIssues?.includes("text") ? (
            <p className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900">
              The previous task failed on product copy quality. Expand the title or
              description input.
            </p>
          ) : null}
        </label>

        {activeSourceTab === "imageUrls" ? (
          <label className="block space-y-2">
            <span className="text-sm font-medium text-zinc-700">Image URLs</span>
            <div className="rounded-2xl border border-dashed border-zinc-300 bg-zinc-50 px-4 py-4">
              <div className="flex flex-wrap items-center gap-3">
                <input
                  aria-label="Upload images"
                  className="block text-sm text-zinc-600 file:mr-4 file:rounded-xl file:border-0 file:bg-zinc-950 file:px-4 file:py-2 file:text-sm file:font-medium file:text-white"
                  multiple
                  onChange={(event) => {
                    const files = Array.from(event.target.files ?? []);
                    setSelectedFiles(files);
                  }}
                  ref={fileInputRef}
                  type="file"
                  accept="image/png,image/jpeg,image/webp,image/gif"
                />
                <Button
                  disabled={selectedFiles.length === 0 || uploadImages.isPending}
                  onClick={async () => {
                    const response = await uploadImages.mutateAsync(selectedFiles);
                    const mergedUrls = [
                      ...parseImageUrls(currentImageUrls ?? ""),
                      ...(response.image_urls ?? []),
                    ];
                    setValue("imageUrls", mergedUrls.join("\n"), {
                      shouldDirty: true,
                    });
                    setSelectedFiles([]);
                    if (fileInputRef.current) {
                      fileInputRef.current.value = "";
                    }
                  }}
                  type="button"
                >
                  {uploadImages.isPending ? "Uploading..." : "Upload selected images"}
                </Button>
              </div>
              <p className="mt-3 text-sm leading-6 text-zinc-500">
                Choose local image files, upload them first, then ListingKit will
                append the returned URLs into the field below.
              </p>
              {selectedFiles.length > 0 ? (
                <p className="mt-2 text-sm text-zinc-700">
                  Selected {selectedFiles.length} file
                  {selectedFiles.length > 1 ? "s" : ""}:{" "}
                  {selectedFiles.map((file) => file.name).join(", ")}
                </p>
              ) : null}
            </div>
            <textarea
              aria-label="Image URLs"
              className="min-h-40 w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
              placeholder={"https://example.com/1.jpg\nhttps://example.com/2.jpg"}
              {...imageUrlsRegistration}
              ref={(element) => {
                imageUrlsRef.current = element;
                imageUrlsRegistration.ref(element);
              }}
            />
            <p className="text-sm leading-6 text-zinc-500">{helperText}</p>
            {errors.imageUrls ? (
              <p className="text-sm text-red-600">{errors.imageUrls.message}</p>
            ) : null}
            {uploadImages.error instanceof Error ? (
              <p className="text-sm text-red-600">{uploadImages.error.message}</p>
            ) : null}
            {fieldIssues?.includes("imageUrls") ? (
              <p className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900">
                The previous task failed on image coverage. Add at least 3 strong
                product images.
              </p>
            ) : null}
          </label>
        ) : null}

        {activeSourceTab === "productUrl" ? (
          <label className="block space-y-2">
            <span className="text-sm font-medium text-zinc-700">Product URL</span>
            <input
              aria-label="Product URL"
              className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
              placeholder="https://detail.1688.com/offer/123456789.html"
              {...productUrlRegistration}
              ref={(element) => {
                productUrlRef.current = element;
                productUrlRegistration.ref(element);
              }}
            />
            <p className="text-sm leading-6 text-zinc-500">
              Optional. Paste a 1688 or other product page URL when you want
              ListingKit to start from the original listing.
            </p>
            {fieldIssues?.includes("productUrl") ? (
              <p className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900">
                The previous task needs a stronger product source. Add a product URL
                so the next run can start from the original listing.
              </p>
            ) : null}
          </label>
        ) : null}

        <fieldset className="space-y-3">
          <legend className="text-sm font-medium text-zinc-700">Platforms</legend>
          <div className="grid gap-3 md:grid-cols-2">
            {platformOptions.map((platform) => (
              <label
                className="flex items-center gap-3 rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-900"
                key={platform.value}
              >
                <input
                  aria-label={platform.label}
                  className="h-4 w-4 rounded border-zinc-300"
                  type="checkbox"
                  value={platform.value}
                  {...register("platforms")}
                />
                <span>{platform.label}</span>
              </label>
            ))}
          </div>
          <p className="text-sm text-zinc-500">
            Selected: {selectedPlatforms?.length ?? 0}
          </p>
          {errors.platforms ? (
            <p className="text-sm text-red-600">{errors.platforms.message}</p>
          ) : null}
        </fieldset>

        {selectedPlatforms?.includes("shein") ? (
          <label className="block space-y-2">
            <span className="text-sm font-medium text-zinc-700">Shein store ID</span>
            <input
              aria-label="Shein store ID"
              className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
              inputMode="numeric"
              placeholder="873"
              {...register("sheinStoreId")}
            />
            <p className="text-sm leading-6 text-zinc-500">
              Optional when this runtime serves a single Shein store. Fill it when multiple Shein stores share the same backend.
            </p>
          </label>
        ) : null}

        <TaskSDSOptions
          enabled={sdsEnabled}
          onEnabledChange={(enabled) => {
            setSDSEnabled(enabled);
            setValue("sdsEnabled", enabled, { shouldDirty: true });
          }}
          variantIdRegistration={register("sdsVariantId")}
          parentProductIdRegistration={register("sdsParentProductId")}
          prototypeGroupIdRegistration={register("sdsPrototypeGroupId")}
          layerIdRegistration={register("sdsLayerId")}
          designTypeRegistration={register("sdsDesignType")}
          fitLevelRegistration={register("sdsFitLevel")}
          resizeModeRegistration={register("sdsResizeMode")}
        />

        <section className="space-y-3 rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="space-y-1">
              <h2 className="text-sm font-medium text-zinc-900">
                Scene generation
              </h2>
              <p className="text-sm leading-6 text-zinc-500">
                Optional. Override the default category scene template with
                structured style controls.
              </p>
            </div>
            <Button
              onClick={() => {
                setShowSceneCustomization((current) => {
                  const next = !current;
                  if (
                    next &&
                    platformSceneDefaults &&
                    !hasAnySceneCustomization({
                      sceneCategory: currentSceneCategory,
                      sceneStyle: currentSceneStyle,
                      backgroundTone: currentBackgroundTone,
                      composition: currentComposition,
                      propsLevel: currentPropsLevel,
                      audienceHint: currentAudienceHint,
                      customSceneHint: currentCustomSceneHint,
                    })
                  ) {
                    setValue("sceneCategory", platformSceneDefaults.sceneCategory ?? "", {
                      shouldDirty: true,
                    });
                    setValue("sceneStyle", platformSceneDefaults.sceneStyle ?? "", {
                      shouldDirty: true,
                    });
                    setValue(
                      "backgroundTone",
                      platformSceneDefaults.backgroundTone ?? "",
                      { shouldDirty: true },
                    );
                    setValue("composition", platformSceneDefaults.composition ?? "", {
                      shouldDirty: true,
                    });
                    setValue("propsLevel", platformSceneDefaults.propsLevel ?? "", {
                      shouldDirty: true,
                    });
                    setValue("audienceHint", platformSceneDefaults.audienceHint ?? "", {
                      shouldDirty: true,
                    });
                    lastAppliedSceneDefaultsRef.current = platformSceneDefaults;
                  }
                  return next;
                });
              }}
              tone="secondary"
              type="button"
            >
              Customize scene generation
            </Button>
          </div>
          {sceneSummary ? (
            <p className="text-sm leading-6 text-zinc-500">{sceneSummary}</p>
          ) : null}

          {showSceneCustomization ? (
            <div className="grid gap-4 md:grid-cols-2">
              <label className="block space-y-2">
                <span className="text-sm font-medium text-zinc-700">
                  Scene category
                </span>
                <select
                  aria-label="Scene category"
                  className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
                  {...register("sceneCategory")}
                >
                  {sceneCategoryOptions.map((option) => (
                    <option key={option.value || "auto"} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>

              <label className="block space-y-2">
                <span className="text-sm font-medium text-zinc-700">
                  Scene style
                </span>
                <select
                  aria-label="Scene style"
                  className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
                  {...register("sceneStyle")}
                >
                  {sceneStyleOptions.map((option) => (
                    <option key={option.value || "auto"} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>

              <label className="block space-y-2">
                <span className="text-sm font-medium text-zinc-700">
                  Background tone
                </span>
                <select
                  aria-label="Background tone"
                  className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
                  {...register("backgroundTone")}
                >
                  {backgroundToneOptions.map((option) => (
                    <option key={option.value || "auto"} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>

              <label className="block space-y-2">
                <span className="text-sm font-medium text-zinc-700">
                  Composition
                </span>
                <select
                  aria-label="Composition"
                  className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
                  {...register("composition")}
                >
                  {compositionOptions.map((option) => (
                    <option key={option.value || "auto"} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>

              <label className="block space-y-2">
                <span className="text-sm font-medium text-zinc-700">
                  Props level
                </span>
                <select
                  aria-label="Props level"
                  className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
                  {...register("propsLevel")}
                >
                  {propsLevelOptions.map((option) => (
                    <option key={option.value || "auto"} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>

              <label className="block space-y-2">
                <span className="text-sm font-medium text-zinc-700">
                  Audience hint
                </span>
                <select
                  aria-label="Audience hint"
                  className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
                  {...register("audienceHint")}
                >
                  {audienceHintOptions.map((option) => (
                    <option key={option.value || "auto"} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>

              <label className="block space-y-2 md:col-span-2">
                <span className="text-sm font-medium text-zinc-700">
                  Custom scene hint
                </span>
                <textarea
                  aria-label="Custom scene hint"
                  className="min-h-28 w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
                  placeholder="Add a short scene preference that complements the category template."
                  {...register("customSceneHint")}
                />
              </label>
            </div>
          ) : null}
        </section>

        {errors.root?.message ? (
          <p className="rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm leading-6 text-red-700">
            {errors.root.message}
          </p>
        ) : null}

        <TaskInputGuidance
          imageCount={imageCount}
          textLength={textLength}
          hasProductUrl={Boolean(currentProductUrl?.trim())}
        />

        <div className="flex flex-wrap gap-3">
          <Button disabled={createTask.isPending} type="submit">
            {createTask.isPending ? "Creating..." : pageCopy.submitLabel}
          </Button>
          <Button tone="secondary" onClick={() => router.push("/")} type="button">
            Cancel
          </Button>
        </div>
      </form>
    </Card>
  );
}
