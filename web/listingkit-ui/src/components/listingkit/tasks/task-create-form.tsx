"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import {
  saveTaskCreateDraft,
  type TaskCreateDraft,
} from "@/components/listingkit/tasks/task-create-draft";
import { TaskInputGuidance } from "@/components/listingkit/tasks/task-input-guidance";
import {
  buildSDSOptions,
  buildSceneOptions,
  type FormValues,
  inferInitialSourceTab,
  parseImageUrls,
  parseOptionalPositiveInt,
  parseSelectedVariantIds,
  platformOptions,
  schema,
  titleFieldCopy,
} from "@/components/listingkit/tasks/task-create-form-model";
import { TaskSDSOptions } from "@/components/listingkit/tasks/task-sds-options";
import { TaskSceneSettingsSection } from "@/components/listingkit/tasks/task-scene-settings-section";
import {
  getPlatformSceneDefaults,
  hasAnySceneCustomization,
  matchesSceneDefaults,
} from "@/components/listingkit/tasks/task-scene-defaults";
import {
  TaskSourceTabs,
  type TaskSourceTab,
} from "@/components/listingkit/tasks/task-source-tabs";
import { useCreateTask } from "@/lib/query/use-create-task";
import { useUploadImages } from "@/lib/query/use-upload-images";
import { loadSDSListingKitMetadata } from "@/lib/sds/product-metadata";
import { useLiveSearchParams } from "@/lib/utils/live-search-params";

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
  const liveSearchParams = useLiveSearchParams();
  const createTask = useCreateTask();
  const uploadImages = useUploadImages();
  const textRef = useRef<HTMLInputElement | null>(null);
  const imageUrlsRef = useRef<HTMLTextAreaElement | null>(null);
  const productUrlRef = useRef<HTMLInputElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const lastAppliedSceneDefaultsRef = useRef<ReturnType<typeof getPlatformSceneDefaults>>(null);
  const lastAppliedSDSQueryKeyRef = useRef<string | null>(null);
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
  const [showAdvancedSettings, setShowAdvancedSettings] = useState(() =>
    Boolean(
      (variant !== "sds" &&
        (initialValues?.sdsEnabled ||
          initialValues?.sdsVariantId ||
          initialValues?.sdsParentProductId ||
          initialValues?.sdsPrototypeGroupId ||
          initialValues?.sdsLayerId)) ||
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

  useEffect(() => {
    if (variant !== "sds") {
      return;
    }

    const nextVariantId = liveSearchParams.get("variantId") ?? "";
    const nextParentProductId = liveSearchParams.get("parentProductId") ?? "";
    const nextPrototypeGroupId = liveSearchParams.get("prototypeGroupId") ?? "";
    const nextLayerId = liveSearchParams.get("layerId") ?? "";
    const hasSDSSelection = Boolean(
      nextVariantId || nextParentProductId || nextPrototypeGroupId || nextLayerId,
    );
    const shouldClearSelection =
      liveSearchParams.get("step") === "select" && !hasSDSSelection;

    if (!hasSDSSelection && !shouldClearSelection) {
      return;
    }

    const key = [
      shouldClearSelection ? "clear" : "set",
      nextVariantId,
      nextParentProductId,
      nextPrototypeGroupId,
      nextLayerId,
    ].join("|");
    if (lastAppliedSDSQueryKeyRef.current === key) {
      return;
    }
    lastAppliedSDSQueryKeyRef.current = key;

    setValue("sdsVariantId", nextVariantId, { shouldValidate: true });
    setValue("sdsParentProductId", nextParentProductId, { shouldValidate: true });
    setValue("sdsPrototypeGroupId", nextPrototypeGroupId, { shouldValidate: true });
    setValue("sdsLayerId", nextLayerId, { shouldValidate: true });
    setValue("sdsEnabled", true, { shouldValidate: true });
    setSDSEnabled(true);
    setShowAdvancedSettings(true);
  }, [liveSearchParams, setValue, variant]);

  const helperText = useMemo(
    () => "可以直接粘贴公网图片链接、上传本地图片，或改用商品链接开始。",
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
          eyebrow: "SDS 同步",
          title: "创建带 SDS 同步的任务",
          description:
            "先完成正常生成流程，再把选中的设计素材同步回 SDS。",
          submitLabel: "创建任务并同步 SDS",
        }
      : {
          eyebrow: "ListingKit",
          title: "创建新任务",
          description:
            "先提供标题、图片或商品链接，再选择要生成的平台。",
          submitLabel: "创建任务",
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
    return `${primaryPlatform} 默认场景：${parts.join(" / ")}`;
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
          ? "rounded-lg border-zinc-200 bg-white p-5 shadow-sm"
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
                "请至少提供商品标题、图片链接或商品链接中的一种。",
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
              message: "启用 SDS 同步时，必须填写有效的 Variant ID。",
            });
            return;
          }
          const sdsMetadata =
            sdsOptions && sdsOptions.variant_id && sdsOptions.parent_product_id
              ? await loadSDSListingKitMetadata({
                  parentProductId: sdsOptions.parent_product_id,
                  variantId: sdsOptions.variant_id,
                  selectedVariantIds: parseSelectedVariantIds(
                    liveSearchParams.get("variantIds"),
                  ),
                })
              : {};
          const enrichedSDSOptions = sdsOptions
            ? {
                ...sdsMetadata,
                ...sdsOptions,
              }
            : undefined;
          const sheinStoreId = parseOptionalPositiveInt(values.sheinStoreId ?? "");
          const options = {
            ...(sceneOptions ? { process_images: true } : {}),
            ...(enrichedSDSOptions && !sceneOptions ? { process_images: false } : {}),
            ...(sceneOptions ? { scene: sceneOptions } : {}),
            ...(enrichedSDSOptions ? { sds: enrichedSDSOptions } : {}),
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
                ? "text-xl font-semibold tracking-tight text-zinc-950"
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
            aria-label="商品标题"
            className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
            placeholder="例如：女士针织开衫"
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
              上一次任务失败在文案质量不足，请补充更完整的标题或描述。
            </p>
          ) : null}
        </label>

        {activeSourceTab === "imageUrls" ? (
          <label className="block space-y-2">
            <span className="text-sm font-medium text-zinc-700">图片链接</span>
            <div className="rounded-2xl border border-dashed border-zinc-300 bg-zinc-50 px-4 py-4">
              <div className="flex flex-wrap items-center gap-3">
                <input
                  aria-label="上传图片"
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
                  {uploadImages.isPending ? "上传中..." : "上传所选图片"}
                </Button>
              </div>
              <p className="mt-3 text-sm leading-6 text-zinc-500">
                可以先上传本地图片，系统会把返回的图片链接自动补到下方输入框里。
              </p>
              {selectedFiles.length > 0 ? (
                <p className="mt-2 text-sm text-zinc-700">
                  已选择 {selectedFiles.length} 个文件：
                  {selectedFiles.map((file) => file.name).join(", ")}
                </p>
              ) : null}
            </div>
            <textarea
              aria-label="图片链接"
              className={`w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 ${
                variant === "sds" ? "min-h-28" : "min-h-40"
              }`}
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
                上一次任务失败在图片覆盖不足，请至少补充 3 张清晰商品图。
              </p>
            ) : null}
          </label>
        ) : null}

        {activeSourceTab === "productUrl" ? (
          <label className="block space-y-2">
            <span className="text-sm font-medium text-zinc-700">商品链接</span>
            <input
              aria-label="商品链接"
              className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
              placeholder="https://detail.1688.com/offer/123456789.html"
              {...productUrlRegistration}
              ref={(element) => {
                productUrlRef.current = element;
                productUrlRegistration.ref(element);
              }}
            />
            <p className="text-sm leading-6 text-zinc-500">
              适合已有原始商品页的场景。支持 1688 等商品链接，系统会从原始商品资料开始处理。
            </p>
            {fieldIssues?.includes("productUrl") ? (
              <p className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900">
                上一次任务缺少足够强的商品来源，建议补充商品链接后再重试。
              </p>
            ) : null}
          </label>
        ) : null}

        <fieldset className="space-y-3">
          <legend className="text-sm font-medium text-zinc-700">目标平台</legend>
          <div className="grid gap-3 md:grid-cols-2">
            {platformOptions.map((platform) => (
              <label
                className="flex items-center gap-3 rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-900"
                key={platform.value}
              >
                <input
                  aria-label={platform.label}
                  className="h-4 w-4 rounded border-zinc-300 accent-zinc-950"
                  defaultChecked={selectedPlatforms?.includes(platform.value)}
                  type="checkbox"
                  value={platform.value}
                  {...register("platforms")}
                />
                <span>{platform.label}</span>
              </label>
            ))}
          </div>
          <p className="text-sm text-zinc-500">
            已选择 {selectedPlatforms?.length ?? 0} 个平台
          </p>
          {errors.platforms ? (
            <p className="text-sm text-red-600">{errors.platforms.message}</p>
          ) : null}
        </fieldset>

        {selectedPlatforms?.includes("shein") ? (
          <label className="block space-y-2">
            <span className="text-sm font-medium text-zinc-700">SHEIN 店铺 ID</span>
            <input
              aria-label="SHEIN 店铺 ID"
              className="w-full rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
              inputMode="numeric"
              placeholder="869"
              {...register("sheinStoreId")}
            />
            <p className="text-sm leading-6 text-zinc-500">
              如果当前环境只对应一个店铺，可以先留空；多个 SHEIN 店铺共用时再填写。
            </p>
          </label>
        ) : null}

        <section className="space-y-3 rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="space-y-1">
              <h2 className="text-sm font-medium text-zinc-900">高级设置</h2>
              <p className="text-sm leading-6 text-zinc-500">
                先填写基础信息；SDS 和场景等高级配置可以稍后再补充。
              </p>
            </div>
            <Button
              onClick={() => setShowAdvancedSettings((current) => !current)}
              tone="secondary"
              type="button"
            >
              {showAdvancedSettings ? "收起高级设置" : "显示高级设置"}
            </Button>
          </div>
        </section>

        {showAdvancedSettings ? (
          <>
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

            <TaskSceneSettingsSection
              currentSceneValues={{
                sceneCategory: currentSceneCategory,
                sceneStyle: currentSceneStyle,
                backgroundTone: currentBackgroundTone,
                composition: currentComposition,
                propsLevel: currentPropsLevel,
                audienceHint: currentAudienceHint,
                customSceneHint: currentCustomSceneHint,
              }}
              lastAppliedSceneDefaultsRef={lastAppliedSceneDefaultsRef}
              platformSceneDefaults={platformSceneDefaults}
              register={register}
              sceneSummary={sceneSummary}
              setShowSceneCustomization={setShowSceneCustomization}
              setValue={setValue}
              showSceneCustomization={showSceneCustomization}
            />
          </>
        ) : null}

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
            {createTask.isPending ? "创建中..." : pageCopy.submitLabel}
          </Button>
          <Button tone="secondary" onClick={() => router.push("/")} type="button">
            返回首页
          </Button>
        </div>
      </form>
    </Card>
  );
}
