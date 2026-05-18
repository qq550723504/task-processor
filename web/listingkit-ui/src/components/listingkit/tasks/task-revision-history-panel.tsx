"use client";

import { useEffect, useMemo, useState } from "react";

import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useTaskRevisionHistory, useTaskRevisionHistoryDetail } from "@/lib/query/use-revision-history";
import type { ListingKitRevisionRecord } from "@/lib/types/listingkit";

function formatRevisionTime(value?: string) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function actionTypeLabel(actionType?: string) {
  switch (actionType) {
    case "restore":
      return "恢复";
    case "edit":
    default:
      return "编辑";
  }
}

function storeStrategyLabel(strategy?: string) {
  switch (strategy) {
    case "priority":
      return "按优先级";
    case "country":
      return "按国家匹配";
    case "manual":
      return "手工优先";
    default:
      return strategy || "";
  }
}

function storeRuleLabel(kind?: string) {
  switch (kind) {
    case "country":
      return "国家规则";
    case "category":
      return "类目规则";
    default:
      return kind || "";
  }
}

function StoreResolutionAudit({
  record,
}: {
  record?: ListingKitRevisionRecord;
}) {
  const resolution = record?.store_resolution;
  if (!resolution?.store_id) {
    return null;
  }
  return (
    <div className="rounded-2xl border border-zinc-200 bg-zinc-50 p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
            店铺快照
          </p>
          <p className="mt-1 text-sm font-medium text-zinc-900">
            SHEIN 店铺 {resolution.store_id}
            {resolution.site ? ` · ${resolution.site}` : ""}
          </p>
          {resolution.reason ? (
            <p className="mt-1 text-sm leading-6 text-zinc-600">{resolution.reason}</p>
          ) : null}
        </div>
        <div className="flex flex-wrap gap-2">
          {resolution.strategy ? (
            <Badge className="rounded-full px-2 py-1 text-[10px]" variant="neutral">
              {storeStrategyLabel(resolution.strategy)}
            </Badge>
          ) : null}
          {resolution.manual_override ? (
            <Badge className="rounded-full px-2 py-1 text-[10px]" variant="success">
              手工指定
            </Badge>
          ) : null}
          {resolution.fallback ? (
            <Badge className="rounded-full px-2 py-1 text-[10px]" variant="warning">
              fallback
            </Badge>
          ) : null}
        </div>
      </div>
      <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-zinc-500">
        {resolution.matched_profile_id ? (
          <span>Profile #{resolution.matched_profile_id}</span>
        ) : null}
        {resolution.resolved_at ? (
          <span>固化时间：{formatRevisionTime(resolution.resolved_at)}</span>
        ) : null}
      </div>
      {resolution.matched_rule_kinds?.length ? (
        <p className="mt-2 text-xs text-zinc-500">
          命中规则：
          {resolution.matched_rule_kinds.map(storeRuleLabel).filter(Boolean).join(" / ")}
        </p>
      ) : null}
    </div>
  );
}

export function TaskRevisionHistoryPanel({ taskId }: { taskId: string }) {
  const historyQuery = useTaskRevisionHistory(taskId, { limit: 5 });
  const items = historyQuery.data?.items ?? [];
  const [selectedRevisionId, setSelectedRevisionId] = useState<string>("");

  useEffect(() => {
    if (!selectedRevisionId && items[0]?.revision_id) {
      setSelectedRevisionId(items[0].revision_id);
    }
  }, [items, selectedRevisionId]);

  const selectedRevision = useMemo(
    () => items.find((item) => item.revision_id === selectedRevisionId),
    [items, selectedRevisionId],
  );

  const detailQuery = useTaskRevisionHistoryDetail(taskId, selectedRevisionId || undefined);
  const detailRecord = detailQuery.data?.record ?? selectedRevision;

  if (historyQuery.isLoading) {
    return (
      <Card className="p-6">
        <p className="text-sm text-zinc-600">正在加载修订历史...</p>
      </Card>
    );
  }

  if (!items.length) {
    return null;
  }

  return (
    <Card className="p-6">
      <div className="space-y-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-zinc-500">
            修订历史
          </p>
          <h2 className="mt-1 text-lg font-semibold text-zinc-950">最近的编辑与恢复记录</h2>
          <p className="mt-1 text-sm leading-6 text-zinc-600">
            这里展示最近的 revision 以及当时绑定的店铺快照，方便对齐编辑历史与提交历史。
          </p>
        </div>

        <div className="grid gap-4 lg:grid-cols-[280px_minmax(0,1fr)]">
          <div className="space-y-2">
            {items.map((item) => {
              const selected = item.revision_id === selectedRevisionId;
              return (
                <Button
                  key={item.revision_id}
                  type="button"
                  variant="outline"
                  className={`h-auto w-full justify-start rounded-2xl px-4 py-3 text-left ${selected ? "border-zinc-900 bg-zinc-50" : "border-zinc-200"}`}
                  onClick={() => setSelectedRevisionId(item.revision_id ?? "")}
                >
                  <div className="space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-sm font-medium text-zinc-900">
                        {item.timeline?.headline || "SHEIN 修订"}
                      </span>
                      <Badge className="rounded-full px-2 py-0.5 text-[10px]" variant="neutral">
                        {actionTypeLabel(item.action_type)}
                      </Badge>
                    </div>
                    <p className="text-xs text-zinc-500">{formatRevisionTime(item.updated_at)}</p>
                    {item.store_resolution?.store_id ? (
                      <p className="text-xs text-zinc-500">
                        店铺 {item.store_resolution.store_id}
                        {item.store_resolution.site ? ` · ${item.store_resolution.site}` : ""}
                      </p>
                    ) : null}
                  </div>
                </Button>
              );
            })}
          </div>

          <div className="space-y-4">
            {detailRecord ? (
              <>
                <div className="rounded-2xl border border-zinc-200 bg-white p-4">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-base font-semibold text-zinc-950">
                      {detailRecord.timeline?.headline || "SHEIN 修订"}
                    </p>
                    <Badge className="rounded-full px-2 py-0.5 text-[10px]" variant="neutral">
                      {actionTypeLabel(detailRecord.action_type)}
                    </Badge>
                  </div>
                  <div className="mt-2 space-y-1 text-sm text-zinc-600">
                    <p>修订时间：{formatRevisionTime(detailRecord.updated_at)}</p>
                    {detailRecord.updated_by ? <p>操作人：{detailRecord.updated_by}</p> : null}
                    {detailRecord.reason ? <p>原因：{detailRecord.reason}</p> : null}
                    {detailRecord.applied_changes?.change_count ? (
                      <p>变更数：{detailRecord.applied_changes.change_count}</p>
                    ) : null}
                  </div>
                </div>
                <StoreResolutionAudit record={detailRecord} />
              </>
            ) : (
              <div className="rounded-2xl border border-zinc-200 bg-zinc-50 p-4 text-sm text-zinc-600">
                暂无可展示的修订详情。
              </div>
            )}
            {detailQuery.isLoading ? (
              <p className="text-xs text-zinc-500">正在加载修订详情...</p>
            ) : null}
          </div>
        </div>
      </div>
    </Card>
  );
}
