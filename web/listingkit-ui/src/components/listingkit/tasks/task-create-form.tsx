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
  buildTaskCreateDefaultValues,
  type FormValues,
  inferInitialSourceTab,
  parseImageUrls,
  parseOptionalPositiveInt,
  parseSelectedVariantIds,
  platformOptions,
  schema,
  taskCreatePageCopy,
  titleFieldCopy,
  type TaskCreateVariant,
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
import {
  TaskImageUrlField,
  TaskProductUrlField,
  TaskTitleField,
} from "@/components/listingkit/tasks/task-create-source-fields";
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
  variant?: TaskCreateVariant;
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
    defaultValues: buildTaskCreateDefaultValues({ initialValues, variant }),
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
  const pageCopy = taskCreatePageCopy(variant);
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
            sheinStoreId: values.sheinStoreId,
            sdsEnabled: values.sdsEnabled,
            sdsVariantId: values.sdsVariantId,
            sdsParentProductId: values.sdsParentProductId,
            sdsPrototypeGroupId: values.sdsPrototypeGroupId,
            sdsLayerId: values.sdsLayerId,
            sdsDesignType: values.sdsDesignType,
            sdsFitLevel: values.sdsFitLevel,
            sdsResizeMode: values.sdsResizeMode,
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

        <TaskTitleField
          errors={errors}
          fieldIssues={fieldIssues}
          inputRef={textRef}
          registration={textRegistration}
          titleCopy={titleCopy}
        />

        {activeSourceTab === "imageUrls" ? (
          <TaskImageUrlField
            currentImageUrls={currentImageUrls}
            errors={errors}
            fieldIssues={fieldIssues}
            fileInputRef={fileInputRef}
            helperText={helperText}
            imageUrlsRef={imageUrlsRef}
            registration={imageUrlsRegistration}
            selectedFiles={selectedFiles}
            setSelectedFiles={setSelectedFiles}
            setValue={setValue}
            uploadImages={uploadImages}
            variant={variant}
          />
        ) : null}

        {activeSourceTab === "productUrl" ? (
          <TaskProductUrlField
            fieldIssues={fieldIssues}
            inputRef={productUrlRef}
            registration={productUrlRegistration}
          />
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
