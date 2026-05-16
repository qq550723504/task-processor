"use client";

import { useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { CheckCircle2, Sparkles, Store } from "lucide-react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Card } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import {
  type TaskCreateDraft,
} from "@/components/listingkit/tasks/task-create-draft";
import { TaskInputGuidance } from "@/components/listingkit/tasks/task-input-guidance";
import {
  buildTaskCreateDefaultValues,
  type FormValues,
  inferInitialSourceTab,
  schema,
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
  useTaskCreateSubmit,
  useTaskCreateWatchedState,
} from "@/components/listingkit/tasks/task-create-form-hooks";
import { useCreateTask } from "@/lib/query/use-create-task";
import { useUploadImages } from "@/lib/query/use-upload-images";
import { useLiveSearchParams } from "@/lib/utils/live-search-params";

function StatusPill({
  label,
  value,
  tone = "default",
}: {
  label: string;
  value: string;
  tone?: "default" | "positive";
}) {
  return (
    <span
      className={`rounded-xl border px-3 py-3 ${
        tone === "positive"
          ? "border-emerald-200 bg-emerald-50 text-emerald-900"
          : "border-zinc-200 bg-white text-zinc-900"
      }`}
    >
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-zinc-500">
        {label}
      </span>
      <span className="ml-2 text-sm font-semibold">{value}</span>
    </span>
  );
}

function ChecklistStat({
  label,
  value,
  helper,
}: {
  label: string;
  value: string;
  helper: string;
}) {
  return (
    <div className="rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-3">
      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-zinc-500">
        {label}
      </div>
      <div className="mt-1 text-base font-semibold text-zinc-950">{value}</div>
      <div className="mt-1 text-xs leading-5 text-zinc-500">{helper}</div>
    </div>
  );
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
      (variant === "sds" &&
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
    variant === "sds",
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

  const {
    currentAudienceHint,
    currentBackgroundTone,
    currentComposition,
    currentCustomSceneHint,
    currentImageUrls,
    currentProductUrl,
    currentPropsLevel,
    currentSceneCategory,
    currentSceneStyle,
    imageCount,
    pageCopy,
    platformSceneDefaults,
    sceneSummary,
    selectedPlatforms,
    textLength,
    titleCopy,
  } = useTaskCreateWatchedState({
    activeSourceTab,
    control,
    variant,
  });

  useTaskCreateSDSQuerySync({
    lastAppliedSDSQueryKeyRef,
    liveSearchParams,
    setSDSEnabled,
    setShowAdvancedSettings,
    setValue,
    variant,
  });

  const helperText = "可以直接粘贴公网图片链接、上传本地图片，或改用商品链接开始。";
  const textRegistration = register("text");
  const imageUrlsRegistration = register("imageUrls");
  const productUrlRegistration = register("productUrl");
  const handleCreateTask = useTaskCreateSubmit({
    clearErrors,
    createTask,
    liveSearchParams,
    router,
    setError,
  });

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

  const hasSourceInput = Boolean(
    (currentProductUrl ?? "").trim() ||
      (currentImageUrls ?? "").trim() ||
      textLength > 0,
  );
  const selectedPlatformCount = selectedPlatforms?.length ?? 0;
  const sourceModeLabel = activeSourceTab === "productUrl" ? "商品链接" : "图片素材";

  return (
    <Card
      className={
        variant === "sds"
          ? "w-full rounded-lg border-zinc-200 bg-white p-5 shadow-sm"
          : "w-full rounded-[1.5rem] border-zinc-200 bg-white shadow-sm"
      }
    >
      <form
        className={variant === "sds" ? "space-y-6" : "space-y-0"}
        onSubmit={handleSubmit(handleCreateTask)}
      >
        <div className="border-b border-zinc-200 bg-zinc-50/60 px-6 py-5 sm:px-8">
          <div className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
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
                    : "text-2xl font-semibold tracking-tight text-zinc-950 sm:text-3xl"
                }
              >
                {pageCopy.title}
              </h1>
              <p className="max-w-2xl text-sm leading-6 text-zinc-600">
                {pageCopy.description}
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              <StatusPill label="来源" value={sourceModeLabel} />
              <StatusPill
                label="平台"
                value={`${selectedPlatformCount} 个`}
                tone={selectedPlatformCount > 0 ? "positive" : "default"}
              />
              <StatusPill
                label="状态"
                value={hasSourceInput ? "可提交" : "待补充"}
                tone={hasSourceInput ? "positive" : "default"}
              />
            </div>
          </div>
        </div>

        <div className="grid gap-6 px-6 py-6 sm:px-8 xl:grid-cols-[minmax(0,1.35fr)_360px]">
          <div className="space-y-6">
            <section className="space-y-6 rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
              <TaskSourceTabs
                activeTab={activeSourceTab}
                embedded
                onTabChange={(tab) => {
                  setActiveSourceTab(tab);
                }}
              />

              <Separator />
              <div>
                <div className="mb-4">
                  <h2 className="text-base font-semibold text-zinc-950">来源信息</h2>
                  <p className="mt-1 text-sm text-zinc-500">
                    先把最核心的商品来源补齐，再决定平台和高级参数。
                  </p>
                </div>

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
              </div>
            </section>

            <TaskAdvancedSettingsToggle
              setShowAdvancedSettings={setShowAdvancedSettings}
              showAdvancedSettings={showAdvancedSettings}
              variant={variant}
            >
              {variant === "sds" ? (
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
              ) : null}

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
                embedded={variant !== "sds"}
                lastAppliedSceneDefaultsRef={lastAppliedSceneDefaultsRef}
                platformSceneDefaults={platformSceneDefaults}
                register={register}
                sceneSummary={sceneSummary}
                setShowSceneCustomization={setShowSceneCustomization}
                setValue={setValue}
                showSceneCustomization={variant === "sds" ? showSceneCustomization : true}
                showToggle={variant === "sds"}
              />
            </TaskAdvancedSettingsToggle>
          </div>

          <aside className="xl:sticky xl:top-6 xl:self-start">
            <section className="space-y-5 rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
              <div className="flex items-center gap-3">
                <span className="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-zinc-100 text-zinc-700">
                  <Store className="h-4 w-4" />
                </span>
                <div>
                  <h2 className="text-base font-semibold text-zinc-950">发布目标</h2>
                  <p className="text-sm text-zinc-500">先确定输出平台，再补充店铺信息。</p>
                </div>
              </div>

              <TaskPlatformFieldset
                errors={errors}
                register={register}
                selectedPlatforms={selectedPlatforms}
              />

              {selectedPlatforms?.includes("shein") ? (
                <TaskSheinStoreField register={register} />
              ) : null}
              <Separator />
              <div>
                <div className="flex items-center gap-3">
                  <span className="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-zinc-100 text-zinc-700">
                    <Sparkles className="h-4 w-4" />
                  </span>
                  <div>
                    <h2 className="text-base font-semibold text-zinc-950">创建检查</h2>
                    <p className="text-sm text-zinc-500">把常见缺口放在提交前一次看完。</p>
                  </div>
                </div>

                <div className="mt-4 grid gap-3">
                <ChecklistStat
                  label="图片"
                  value={`${imageCount} 张`}
                  helper={imageCount >= 3 ? "已达到常用建议值。" : "建议至少补到 3 张。"}
                />
                <ChecklistStat
                  label="标题"
                  value={`${textLength} 字`}
                  helper={textLength >= 50 ? "标题长度基本够用。" : "建议标题或文案更完整。"}
                />
                <ChecklistStat
                  label="商品链接"
                  value={currentProductUrl?.trim() ? "已提供" : "未提供"}
                  helper={currentProductUrl?.trim() ? "可用于补强来源信息。" : "有链接时识别通常更稳。"}
                />
                </div>

                <Separator className="my-4" />
                <div>
                  <TaskInputGuidance
                    embedded
                    imageCount={imageCount}
                    textLength={textLength}
                    hasProductUrl={Boolean(currentProductUrl?.trim())}
                  />
                </div>

                {selectedPlatforms?.includes("shein") ? (
                  <div className="mt-4 rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-600">
                    SHEIN 已启用时，如有多个店铺共用环境，建议补充店铺 ID。
                  </div>
                ) : null}
              </div>

              <Separator />
              <div>
                <div className="mb-4 flex items-center gap-3">
                  <span className="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-50 text-emerald-700">
                    <CheckCircle2 className="h-4 w-4" />
                  </span>
                  <div>
                    <h2 className="text-base font-semibold text-zinc-950">创建动作</h2>
                    <p className="text-sm text-zinc-500">确认后直接进入任务状态页。</p>
                  </div>
                </div>
                <TaskCreateFormActions
                  isCreating={createTask.isPending}
                  onBack={() => router.push("/")}
                  submitLabel={pageCopy.submitLabel}
                />
                {errors.root?.message ? (
                  <Alert className="mt-4" variant="destructive">
                    <AlertDescription>{errors.root.message}</AlertDescription>
                  </Alert>
                ) : null}
              </div>
            </section>
          </aside>
        </div>
      </form>
    </Card>
  );
}
