"use client";

import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { getImageAgentTaskAssets, launchImageAgentTaskRun } from "@/lib/api/image-agent";
import type { ImageAgentAuthorizedAsset } from "@/lib/types/image-agent";

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
  const [sourceId, setSourceId] = useState("");
  const [styleIds, setStyleIds] = useState<string[]>([]);

  const preflightKey = open && targetPlatform
    ? `${encodeURIComponent(taskId)}|${encodeURIComponent(targetPlatform)}`
    : null;
  const [preflight, setPreflight] = useState<{
    key: string;
    sources?: ImageAgentAuthorizedAsset[];
    styles?: ImageAgentAuthorizedAsset[];
    error?: string;
  }>();

  useEffect(() => {
    if (!preflightKey || !targetPlatform) return;
    const controller = new AbortController();
    let active = true;
    getImageAgentTaskAssets(taskId, targetPlatform, controller.signal).then(
      (assets) => {
        if (active) {
          setPreflight({ key: preflightKey, sources: assets.sources, styles: assets.styles });
        }
      },
      (reason) => {
        if (active) {
          setPreflight({
            key: preflightKey,
            error: reason instanceof Error ? reason.message : "加载任务素材失败",
          });
        }
      },
    );
    return () => {
      active = false;
      controller.abort();
    };
  }, [preflightKey, taskId, targetPlatform]);

  const currentPreflight = preflight?.key === preflightKey ? preflight : undefined;
  const assetsLoading = preflightKey !== null && !currentPreflight;
  const sources = currentPreflight?.sources;
  const styles = currentPreflight?.styles;
  const assetsError = currentPreflight?.error;

  if (!open) {
    return (
      <Button onClick={() => setOpen(true)} type="button" variant="secondary">
        创建图片方案
      </Button>
    );
  }

  const toggleStyle = (id: string) => {
    setStyleIds((current) =>
      current.includes(id) ? current.filter((item) => item !== id) : [...current, id],
    );
  };

  const launch = async () => {
    if (creating || !targetPlatform || !sceneCategory || !sourceId) return;
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
        source_asset_id: sourceId,
        style_asset_ids: styleIds.length > 0 ? styleIds : undefined,
      });
      onLaunched(created.run_id);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "创建图片方案失败");
    } finally {
      setCreating(false);
    }
  };

  const assetsReady = !assetsLoading && sources !== undefined;

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
      {assetsError ? (
        <p className="text-sm text-destructive" role="alert">
          {assetsError}
        </p>
      ) : null}
      {assetsLoading ? <p className="text-sm text-muted-foreground">正在加载任务素材…</p> : null}
      {assetsReady && sources.length === 0 ? (
        <p className="text-sm text-destructive" role="alert">
          当前任务没有可选的主素材，无法创建图片方案。
        </p>
      ) : null}
      {assetsReady && sources.length > 0 ? (
        <fieldset className="space-y-2">
          <legend className="text-sm font-medium text-foreground">主素材（必选）</legend>
          <div className="grid gap-2 sm:grid-cols-2">
            {sources.map((asset) => (
              <label className="flex items-center gap-2 rounded-lg border border-border p-2 text-sm" key={asset.id}>
                <input
                  checked={sourceId === asset.id}
                  name="image-agent-source-asset"
                  onChange={() => setSourceId(asset.id)}
                  type="radio"
                  value={asset.id}
                />
                {asset.display_url ? (
                  <img
                    alt={asset.label || asset.id}
                    className="h-12 w-12 rounded-md object-cover"
                    src={asset.display_url}
                  />
                ) : null}
                <span>{asset.label || asset.id}</span>
              </label>
            ))}
          </div>
        </fieldset>
      ) : null}
      {assetsReady && styles && styles.length > 0 ? (
        <fieldset className="space-y-2">
          <legend className="text-sm font-medium text-foreground">风格参考（可选）</legend>
          <div className="grid gap-2 sm:grid-cols-2">
            {styles.map((asset) => (
              <label className="flex items-center gap-2 rounded-lg border border-border p-2 text-sm" key={asset.id}>
                <input
                  checked={styleIds.includes(asset.id)}
                  onChange={() => toggleStyle(asset.id)}
                  type="checkbox"
                  value={asset.id}
                />
                {asset.display_url ? (
                  <img
                    alt={asset.label || asset.id}
                    className="h-12 w-12 rounded-md object-cover"
                    src={asset.display_url}
                  />
                ) : null}
                <span>{asset.label || asset.id}</span>
              </label>
            ))}
          </div>
        </fieldset>
      ) : null}
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
      <Button
        disabled={!targetPlatform || !sceneCategory || !sourceId || creating || assetsLoading}
        onClick={launch}
        type="button"
      >
        {creating ? "正在创建…" : "开始生成"}
      </Button>
    </section>
  );
}
