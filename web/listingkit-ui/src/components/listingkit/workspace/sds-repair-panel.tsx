"use client";

import { useState } from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { useRepairAndRetryTaskSDS, useTaskSDSRepair } from "@/lib/query/use-sds-repair";

export function SDSRepairPanel({ taskId, open, onClose }: { taskId: string; open: boolean; onClose: () => void }) {
  const session = useTaskSDSRepair(taskId, open);
  const repair = useRepairAndRetryTaskSDS(taskId);
  const [selected, setSelected] = useState<Record<number, string>>({});
  if (!open) return null;
  if (session.isLoading) return <Card className="p-5 text-sm text-muted-foreground">正在读取当前 SDS 图层…</Card>;
  if (session.isError || !session.data) return <Card className="border-red-200 p-5 text-sm text-red-700">无法读取当前 SDS 图层，请稍后重试。</Card>;
  const complete = session.data.variants.every((variant) => Boolean(selected[variant.variant_id]));
  return <Card className="space-y-4 border-amber-200 p-5">
    <div><h2 className="text-lg font-semibold">修复并重试 SDS</h2><p className="mt-1 text-sm text-muted-foreground">请为每个变体明确选择当前可用图层；不会创建新任务。</p></div>
    {session.data.variants.map((variant) => <div className="space-y-2 rounded-xl border p-3" key={variant.variant_id}>
      <p className="text-sm font-medium">{variant.color || "未命名颜色"}{variant.size ? ` / ${variant.size}` : ""}{variant.variant_sku ? `（${variant.variant_sku}）` : ""}</p>
      <p className="text-xs text-red-700">原图层：{variant.old_layer_id || "未记录"}</p>
      <Select aria-label={`${variant.color || variant.variant_id} 图层`} value={selected[variant.variant_id] ?? ""} onChange={(event) => setSelected((current) => ({ ...current, [variant.variant_id]: event.target.value }))}>
        <option value="">请选择当前图层</option>
        {variant.layers.map((layer) => <option key={layer.id} value={layer.id}>{layer.name ? `${layer.name} (${layer.id})` : layer.id}</option>)}
      </Select>
    </div>)}
    {repair.error ? <p className="text-sm text-red-700">修复失败，请核对所选图层后重试。</p> : null}
    <div className="flex gap-2"><Button disabled={!complete || repair.isPending} onClick={() => repair.mutate({ variants: session.data.variants.map((variant) => ({ variant_id: variant.variant_id, layer_id: selected[variant.variant_id] })) }, { onSuccess: onClose })} type="button">{repair.isPending ? "正在重试…" : "确认修复并重试"}</Button><Button onClick={onClose} type="button" variant="outline">取消</Button></div>
  </Card>;
}
