import type { Dispatch, SetStateAction } from "react";
import type { UseFormRegister, UseFormSetValue } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import {
  audienceHintOptions,
  backgroundToneOptions,
  compositionOptions,
  type FormValues,
  propsLevelOptions,
  sceneCategoryOptions,
  sceneStyleOptions,
} from "@/components/listingkit/tasks/task-create-form-model";
import {
  hasAnySceneCustomization,
  type TaskSceneDraftValues,
} from "@/components/listingkit/tasks/task-scene-defaults";

export function TaskSceneSettingsSection({
  currentSceneValues,
  embedded = false,
  lastAppliedSceneDefaultsRef,
  platformSceneDefaults,
  register,
  sceneSummary,
  setShowSceneCustomization,
  setValue,
  showSceneCustomization,
  showToggle = true,
}: {
  currentSceneValues: TaskSceneDraftValues;
  embedded?: boolean;
  lastAppliedSceneDefaultsRef: { current: TaskSceneDraftValues | null };
  platformSceneDefaults: TaskSceneDraftValues | null;
  register: UseFormRegister<FormValues>;
  sceneSummary: string | null;
  setShowSceneCustomization: Dispatch<SetStateAction<boolean>>;
  setValue: UseFormSetValue<FormValues>;
  showSceneCustomization: boolean;
  showToggle?: boolean;
}) {
  const sceneFieldsVisible = showToggle ? showSceneCustomization : true;

  return (
    <section
      className={
        embedded
          ? "space-y-4"
          : "space-y-3 rounded-2xl border border-border bg-muted px-4 py-4"
      }
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="space-y-1">
          <h2 className="text-sm font-medium text-foreground">场景生成设置</h2>
          <p className="text-sm leading-6 text-muted-foreground">
            如果你希望更精细地控制画面风格，可以在这里补充场景偏好。
          </p>
        </div>
        {showToggle ? (
          <Button
            onClick={() => {
              setShowSceneCustomization((current) => {
                const next = !current;
                if (
                  next &&
                  platformSceneDefaults &&
                  !hasAnySceneCustomization(currentSceneValues)
                ) {
                  setValue("sceneCategory", platformSceneDefaults.sceneCategory ?? "", {
                    shouldDirty: true,
                  });
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
                }
                return next;
              });
            }}
            variant="secondary"
            type="button"
          >
            {showSceneCustomization ? "收起场景设置" : "显示场景设置"}
          </Button>
        ) : null}
      </div>
      {sceneSummary ? (
        <p className="text-sm leading-6 text-muted-foreground">{sceneSummary}</p>
      ) : null}

      {sceneFieldsVisible ? (
        <div className="grid gap-4 md:grid-cols-2">
          <Label className="block space-y-2">
            <span className="text-sm font-medium text-muted-foreground">场景类目</span>
            <Select
              aria-label="场景类目"
              className="rounded-2xl px-4 py-3"
              {...register("sceneCategory")}
            >
              {sceneCategoryOptions.map((option) => (
                <option key={option.value || "auto"} value={option.value}>
                  {option.label}
                </option>
              ))}
            </Select>
          </Label>

          <Label className="block space-y-2">
            <span className="text-sm font-medium text-muted-foreground">场景风格</span>
            <Select
              aria-label="场景风格"
              className="rounded-2xl px-4 py-3"
              {...register("sceneStyle")}
            >
              {sceneStyleOptions.map((option) => (
                <option key={option.value || "auto"} value={option.value}>
                  {option.label}
                </option>
              ))}
            </Select>
          </Label>

          <Label className="block space-y-2">
            <span className="text-sm font-medium text-muted-foreground">背景色调</span>
            <Select
              aria-label="背景色调"
              className="rounded-2xl px-4 py-3"
              {...register("backgroundTone")}
            >
              {backgroundToneOptions.map((option) => (
                <option key={option.value || "auto"} value={option.value}>
                  {option.label}
                </option>
              ))}
            </Select>
          </Label>

          <Label className="block space-y-2">
            <span className="text-sm font-medium text-muted-foreground">构图方式</span>
            <Select
              aria-label="构图方式"
              className="rounded-2xl px-4 py-3"
              {...register("composition")}
            >
              {compositionOptions.map((option) => (
                <option key={option.value || "auto"} value={option.value}>
                  {option.label}
                </option>
              ))}
            </Select>
          </Label>

          <Label className="block space-y-2">
            <span className="text-sm font-medium text-muted-foreground">道具强度</span>
            <Select
              aria-label="道具强度"
              className="rounded-2xl px-4 py-3"
              {...register("propsLevel")}
            >
              {propsLevelOptions.map((option) => (
                <option key={option.value || "auto"} value={option.value}>
                  {option.label}
                </option>
              ))}
            </Select>
          </Label>

          <Label className="block space-y-2">
            <span className="text-sm font-medium text-muted-foreground">风格倾向</span>
            <Select
              aria-label="风格倾向"
              className="rounded-2xl px-4 py-3"
              {...register("audienceHint")}
            >
              {audienceHintOptions.map((option) => (
                <option key={option.value || "auto"} value={option.value}>
                  {option.label}
                </option>
              ))}
            </Select>
          </Label>

          <Label className="block space-y-2 md:col-span-2">
            <span className="text-sm font-medium text-muted-foreground">自定义场景说明</span>
            <Textarea
              aria-label="自定义场景说明"
              className="min-h-28 rounded-2xl px-4 py-3"
              placeholder="补充一句你希望系统遵循的画面要求。"
              {...register("customSceneHint")}
            />
          </Label>
        </div>
      ) : null}
    </section>
  );
}
