"use client";

import { useState } from "react";

import { Button } from "@/components/ui/button";
import { launchImageAgentTaskRun } from "@/lib/api/image-agent";

const sceneCategoryChoices = [
  { value: "shoes", label: "鞋履" },
  { value: "jewelry", label: "饰品" },
  { value: "bags", label: "箱包" },
] as const;

const defaultLaunchCountry = "us";

export function ImageAgentTaskRunLauncher({
  taskId,
  targetPlatform,
  country,
  onLaunched,
}: {
  taskId: string;
  targetPlatform?: string;
  country?: string;
  onLaunched: (runId: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [sceneCategory, setSceneCategory] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string>();

  if (!open) {
    return (
      <Button onClick={() => setOpen(true)} type="button" variant="secondary">
        创建图片方案
      </Button>
    );
  }

  const launch = async () => {
    if (creating || !targetPlatform || !sceneCategory) return;
    setCreating(true);
    setError(undefined);
    try {
      const created = await launchImageAgentTaskRun({
        business_task_id: taskId,
        target_platform: targetPlatform,
        image_policy_context: {
          country: country?.trim() ? country.trim().toLowerCase() : defaultLaunchCountry,
          family: "default",
          scene_category: sceneCategory,
        },
      });
      onLaunched(created.run_id);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "创建图片方案失败");
    } finally {
      setCreating(false);
    }
  };

  return (
    <section aria-label="创建图片方案" className="space-y-4 rounded-2xl border border-border bg-card p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="font-semibold text-foreground">创建图片方案</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            为当前任务启动图片 Agent 生成；创建后可在运行面板中调整素材与计划。
          </p>
        </div>
        <Button onClick={() => setOpen(false)} size="sm" type="button" variant="ghost">
          取消
        </Button>
      </div>
      {targetPlatform ? null : (
        <p className="text-sm text-destructive" role="alert">
          当前任务未确定目标平台，无法创建图片方案。
        </p>
      )}
      <fieldset className="space-y-2">
        <legend className="text-sm font-medium text-foreground">拍摄场景</legend>
        <div className="flex flex-wrap gap-3">
          {sceneCategoryChoices.map((choice) => (
            <label className="flex items-center gap-2 text-sm" key={choice.value}>
              <input
                checked={sceneCategory === choice.value}
                name="image-agent-scene-category"
                onChange={() => setSceneCategory(choice.value)}
                type="radio"
                value={choice.value}
              />
              <span>{choice.label}</span>
            </label>
          ))}
        </div>
      </fieldset>
      {error ? (
        <p className="text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}
      <Button disabled={!targetPlatform || !sceneCategory || creating} onClick={launch} type="button">
        {creating ? "正在创建…" : "开始生成"}
      </Button>
    </section>
  );
}
