import { useCallback, useEffect, useMemo } from "react";
import type { MutableRefObject, RefObject } from "react";
import type {
  Control,
  UseFormClearErrors,
  UseFormSetError,
  UseFormSetValue,
} from "react-hook-form";
import { useWatch } from "react-hook-form";

import {
  saveTaskCreateDraft,
} from "@/components/listingkit/tasks/task-create-draft";
import {
  buildTaskCreateSubmission,
} from "@/components/listingkit/tasks/task-create-submit";
import type {
  FormValues,
  TaskCreateVariant,
} from "@/components/listingkit/tasks/task-create-form-model";
import {
  parseImageUrls,
  parseSelectedVariantIds,
  taskCreatePageCopy,
  titleFieldCopy,
} from "@/components/listingkit/tasks/task-create-form-model";
import {
  getPlatformSceneDefaults,
  hasAnySceneCustomization,
  matchesSceneDefaults,
  type TaskSceneDraftValues,
} from "@/components/listingkit/tasks/task-scene-defaults";
import type { TaskSourceTab } from "@/components/listingkit/tasks/task-source-tabs";
import type {
  CreateListingKitTaskRequest,
  CreateListingKitTaskResponse,
} from "@/lib/types/listingkit";

type SearchParamReader = {
  get: (name: string) => string | null;
};

export function useTaskCreateWatchedState({
  activeSourceTab,
  control,
  variant,
}: {
  activeSourceTab: TaskSourceTab;
  control: Control<FormValues>;
  variant: TaskCreateVariant;
}) {
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
  const imageCount = useMemo(
    () => parseImageUrls(currentImageUrls ?? "").length,
    [currentImageUrls],
  );
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

  return {
    currentAudienceHint,
    currentBackgroundTone,
    currentComposition,
    currentCustomSceneHint,
    currentImageUrls,
    currentProductUrl,
    currentPropsLevel,
    currentSceneCategory,
    currentSceneStyle,
    currentText,
    imageCount,
    pageCopy: taskCreatePageCopy(variant),
    platformSceneDefaults,
    sceneSummary,
    selectedPlatforms,
    textLength: (currentText ?? "").trim().length,
    titleCopy: titleFieldCopy(activeSourceTab),
  };
}

export function useTaskCreateSubmit({
  clearErrors,
  createTask,
  liveSearchParams,
  router,
  setError,
}: {
  clearErrors: UseFormClearErrors<FormValues>;
  createTask: {
    mutateAsync: (
      request: CreateListingKitTaskRequest,
    ) => Promise<CreateListingKitTaskResponse>;
  };
  liveSearchParams: SearchParamReader;
  router: { push: (href: string) => void };
  setError: UseFormSetError<FormValues>;
}) {
  return useCallback(
    async (values: FormValues) => {
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
    },
    [clearErrors, createTask, liveSearchParams, router, setError],
  );
}

export function useTaskCreateSDSQuerySync({
  lastAppliedSDSQueryKeyRef,
  liveSearchParams,
  setSDSEnabled,
  setShowAdvancedSettings,
  setValue,
  variant,
}: {
  lastAppliedSDSQueryKeyRef: MutableRefObject<string | null>;
  liveSearchParams: SearchParamReader;
  setSDSEnabled: (value: boolean) => void;
  setShowAdvancedSettings: (value: boolean) => void;
  setValue: UseFormSetValue<FormValues>;
  variant: TaskCreateVariant;
}) {
  useEffect(() => {
    if (variant !== "sds") {
      return;
    }

    const nextVariantId = liveSearchParams.get("variantId") ?? "";
    const nextParentProductId = liveSearchParams.get("parentProductId") ?? "";
    const nextPrototypeGroupId = liveSearchParams.get("prototypeGroupId") ?? "";
    const nextLayerId = liveSearchParams.get("layerId") ?? "";
    const hasSDSSelection = Boolean(
      nextVariantId ||
        nextParentProductId ||
        nextPrototypeGroupId ||
        nextLayerId,
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

    const timer = window.setTimeout(() => {
      setValue("sdsVariantId", nextVariantId, { shouldValidate: true });
      setValue("sdsParentProductId", nextParentProductId, {
        shouldValidate: true,
      });
      setValue("sdsPrototypeGroupId", nextPrototypeGroupId, {
        shouldValidate: true,
      });
      setValue("sdsLayerId", nextLayerId, { shouldValidate: true });
      setValue("sdsEnabled", true, { shouldValidate: true });
      setSDSEnabled(true);
      setShowAdvancedSettings(true);
    }, 0);

    return () => {
      window.clearTimeout(timer);
    };
  }, [
    lastAppliedSDSQueryKeyRef,
    liveSearchParams,
    setSDSEnabled,
    setShowAdvancedSettings,
    setValue,
    variant,
  ]);
}

export function useTaskCreateSceneDefaults({
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
}: {
  currentAudienceHint?: string;
  currentBackgroundTone?: string;
  currentComposition?: string;
  currentCustomSceneHint?: string;
  currentPropsLevel?: string;
  currentSceneCategory?: string;
  currentSceneStyle?: string;
  lastAppliedSceneDefaultsRef: MutableRefObject<TaskSceneDraftValues | null>;
  platformSceneDefaults: TaskSceneDraftValues | null;
  setValue: UseFormSetValue<FormValues>;
  showSceneCustomization: boolean;
}) {
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
      matchesSceneDefaults(
        currentSceneValues,
        lastAppliedSceneDefaultsRef.current,
      );
    if (!canApplyDefaults) {
      return;
    }

    const timer = window.setTimeout(() => {
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
    }, 0);

    return () => {
      window.clearTimeout(timer);
    };
  }, [
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
  ]);
}

export function useTaskCreateFocus({
  activeSourceTab,
  imageUrlsRef,
  initialFocus,
  productUrlRef,
  textRef,
}: {
  activeSourceTab: TaskSourceTab;
  imageUrlsRef: RefObject<HTMLTextAreaElement | null>;
  initialFocus?: "text" | "imageUrls" | "productUrl";
  productUrlRef: RefObject<HTMLInputElement | null>;
  textRef: RefObject<HTMLInputElement | null>;
}) {
  useEffect(() => {
    if (initialFocus === "text") {
      textRef.current?.focus();
    }
  }, [initialFocus, textRef]);

  useEffect(() => {
    if (activeSourceTab === "productUrl") {
      productUrlRef.current?.focus();
      return;
    }
    imageUrlsRef.current?.focus();
  }, [activeSourceTab, imageUrlsRef, productUrlRef]);
}
