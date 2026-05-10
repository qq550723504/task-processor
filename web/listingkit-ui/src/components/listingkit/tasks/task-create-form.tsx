"use client";

import { useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Card } from "@/components/shared/card";
import {
  saveTaskCreateDraft,
  type TaskCreateDraft,
} from "@/components/listingkit/tasks/task-create-draft";
import { TaskInputGuidance } from "@/components/listingkit/tasks/task-input-guidance";
import {
  buildTaskCreateDefaultValues,
  type FormValues,
  inferInitialSourceTab,
  parseImageUrls,
  parseSelectedVariantIds,
  schema,
  taskCreatePageCopy,
  titleFieldCopy,
  type TaskCreateVariant,
} from "@/components/listingkit/tasks/task-create-form-model";
import { TaskSDSOptions } from "@/components/listingkit/tasks/task-sds-options";
import { TaskSceneSettingsSection } from "@/components/listingkit/tasks/task-scene-settings-section";
import {
  getPlatformSceneDefaults,
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
import {
  TaskAdvancedSettingsToggle,
  TaskCreateFormActions,
  TaskPlatformFieldset,
  TaskSheinStoreField,
} from "@/components/listingkit/tasks/task-create-form-sections";
import {
  useTaskCreateFocus,
  useTaskCreateSDSQuerySync,
  useTaskCreateSceneDefaults,
} from "@/components/listingkit/tasks/task-create-form-hooks";
import { buildTaskCreateSubmission } from "@/components/listingkit/tasks/task-create-submit";
import { useCreateTask } from "@/lib/query/use-create-task";
import { useUploadImages } from "@/lib/query/use-upload-images";
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

  useTaskCreateSDSQuerySync({
    lastAppliedSDSQueryKeyRef,
    liveSearchParams,
    setSDSEnabled,
    setShowAdvancedSettings,
    setValue,
    variant,
  });

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

  useTaskCreateSceneDefaults({
    currentAudienceHint,
    currentBackgroundTone,
    currentComposition,
    currentCustomSceneHint,
    currentPropsLevel,
    currentSceneCategory,
    currentSceneStyle,
    lastAppliedSceneDefaultsRef,
    platformSceneDefaults,
    setValue,
    showSceneCustomization,
  });

  useTaskCreateFocus({
    activeSourceTab,
    imageUrlsRef,
    initialFocus,
    productUrlRef,
    textRef,
  });

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
          const submission = await buildTaskCreateSubmission({
            selectedVariantIds: parseSelectedVariantIds(
              liveSearchParams.get("variantIds"),
            ),
            values,
          });
          if (!submission.ok) {
            setError("root", {
              message: submission.message,
            });
            return;
          }
          clearErrors("root");
          const task = await createTask.mutateAsync(submission.request);
          saveTaskCreateDraft(task.task_id, submission.draft);
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

        <TaskPlatformFieldset
          errors={errors}
          register={register}
          selectedPlatforms={selectedPlatforms}
        />

        {selectedPlatforms?.includes("shein") ? (
          <TaskSheinStoreField register={register} />
        ) : null}

        <TaskAdvancedSettingsToggle
          setShowAdvancedSettings={setShowAdvancedSettings}
          showAdvancedSettings={showAdvancedSettings}
        />

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

        <TaskCreateFormActions
          isCreating={createTask.isPending}
          onBack={() => router.push("/")}
          submitLabel={pageCopy.submitLabel}
        />
      </form>
    </Card>
  );
}
