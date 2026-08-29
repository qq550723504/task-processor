"use client";

import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  createImageAgentWorkspaceRun,
  getImageAgentWorkspaceAssets,
} from "@/lib/api/image-agent";
import type { ImageAgentWorkspaceAssets } from "@/lib/types/image-agent";

export function ImageAgentLaunchPanel({
  taskId,
  targetPlatform,
  onCreated,
}: {
  taskId: string;
  targetPlatform?: string;
  onCreated: (runId: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [assets, setAssets] = useState<ImageAgentWorkspaceAssets>();
  const [sourceID, setSourceID] = useState("");
  const [styleIDs, setStyleIDs] = useState<string[]>([]);
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    setLoading(true);
    setError(undefined);
    setAssets(undefined);
    setSourceID("");
    setStyleIDs([]);
    getImageAgentWorkspaceAssets(taskId, targetPlatform, controller.signal)
      .then(setAssets)
      .catch((reason: unknown) => {
        if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : "无法读取任务图片素材");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [open, targetPlatform, taskId]);

  const toggleStyle = (id: string) => setStyleIDs((current) => current.includes(id) ? current.filter((value) => value !== id) : [...current, id]);
  const create = async () => {
    if (!sourceID || creating) return;
    setCreating(true);
    setError(undefined);
    try {
      const created = await createImageAgentWorkspaceRun(taskId, {
        ...(assets?.target_platform ? { target_platform: assets.target_platform } : {}),
        source_asset_id: sourceID,
        style_asset_ids: styleIDs,
      });
      onCreated(created.run_id);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "创建图片方案失败");
    } finally {
      setCreating(false);
    }
  };

  if (!open) {
    return <Button onClick={() => setOpen(true)} type="button" variant="secondary">创建图片方案</Button>;
  }

  return (
    <section className="space-y-4 rounded-2xl border border-border bg-card p-4" aria-label="创建图片方案">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="font-semibold text-foreground">创建图片方案</h2>
          <p className="mt-1 text-sm text-muted-foreground">选择一个商品来源素材；风格参考可选。</p>
        </div>
        <Button onClick={() => setOpen(false)} size="sm" type="button" variant="ghost">取消</Button>
      </div>
      {loading ? <p className="text-sm text-muted-foreground">正在读取可用素材…</p> : null}
      {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
      {assets ? <>
        <fieldset className="space-y-2">
          <legend className="text-sm font-medium text-foreground">商品来源素材</legend>
          {assets.source_assets.map((asset) => <label className="flex items-center gap-2 text-sm" key={asset.id}>
            <input checked={sourceID === asset.id} name="image-agent-source" onChange={() => setSourceID(asset.id)} type="radio" value={asset.id} />
            <span>{asset.label}</span>
          </label>)}
        </fieldset>
        {assets.style_candidates.length > 0 ? <fieldset className="space-y-2">
          <legend className="text-sm font-medium text-foreground">风格参考（可选）</legend>
          {assets.style_candidates.map((asset) => <label className="flex items-center gap-2 text-sm" key={asset.id}>
            <input checked={styleIDs.includes(asset.id)} onChange={() => toggleStyle(asset.id)} type="checkbox" value={asset.id} />
            <span>{asset.label}</span>
          </label>)}
        </fieldset> : null}
        <Button disabled={!sourceID || creating} onClick={create} type="button">{creating ? "正在创建…" : "开始生成"}</Button>
      </> : null}
    </section>
  );
}
