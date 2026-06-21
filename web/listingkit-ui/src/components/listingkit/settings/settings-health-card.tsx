"use client";

import {
  AlertTriangle,
  CheckCircle2,
  CircleHelp,
  type LucideIcon,
  XCircle,
} from "lucide-react";

import { ListingKitSettingsSection } from "@/components/listingkit/settings/listingkit-settings-section";
import { useListingKitSettingsHealth } from "@/lib/query/use-listingkit-settings-metadata";
import type {
  ListingKitSettingsHealthItem,
  ListingKitSettingsHealthStatus,
} from "@/lib/types/listingkit";

const statusCopy: Record<
  ListingKitSettingsHealthStatus,
  { label: string; className: string; Icon: LucideIcon }
> = {
  ready: {
    label: "配置可用",
    className: "border-emerald-200 bg-emerald-50 text-emerald-800",
    Icon: CheckCircle2,
  },
  warning: {
    label: "需要关注",
    className: "border-amber-200 bg-amber-50 text-amber-900",
    Icon: AlertTriangle,
  },
  blocked: {
    label: "存在阻断项",
    className: "border-rose-200 bg-rose-50 text-rose-800",
    Icon: XCircle,
  },
  unknown: {
    label: "待接入探针",
    className: "border-slate-200 bg-slate-50 text-slate-700",
    Icon: CircleHelp,
  },
};

export function SettingsHealthCard() {
  const health = useListingKitSettingsHealth();
  const status = health.data?.status ?? "unknown";
  const summary = statusCopy[status];

  return (
    <ListingKitSettingsSection
      id="health"
      eyebrow="Preflight"
      title="配置健康检查"
      description="集中检查新任务和 SHEIN 提交流程依赖的 AI、SHEIN、SDS、图片模型、价格规则与对象存储配置。"
      actions={
        <span
          className={[
            "inline-flex h-9 items-center rounded-xl border px-3 text-xs font-semibold",
            summary.className,
          ].join(" ")}
        >
          {summary.label}
        </span>
      }
    >
      {health.isPending ? (
        <div className="rounded-2xl border border-border bg-muted/60 p-4 text-sm text-muted-foreground">
          正在检查配置...
        </div>
      ) : health.isError ? (
        <div className="rounded-2xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-800">
          配置健康检查读取失败，请稍后重试或查看后端日志。
        </div>
      ) : (
        <div className="grid gap-3 lg:grid-cols-2">
          {(health.data?.items ?? []).map((item) => (
            <HealthItemCard key={item.key} item={item} />
          ))}
        </div>
      )}
    </ListingKitSettingsSection>
  );
}

function HealthItemCard({ item }: { item: ListingKitSettingsHealthItem }) {
  const copy = statusCopy[item.status];
  const Icon = copy.Icon;
  return (
    <article className="rounded-2xl border border-border bg-background p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="text-sm font-semibold text-foreground">{item.label}</div>
          <p className="mt-1 text-sm leading-5 text-muted-foreground">{item.message}</p>
        </div>
        <span
          className={[
            "inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-1 text-[11px] font-semibold",
            copy.className,
          ].join(" ")}
        >
          <Icon className="h-3.5 w-3.5" />
          {copy.label}
        </span>
      </div>
      {item.impact?.length ? (
        <div className="mt-3 rounded-xl border border-sky-200 bg-sky-50 px-3 py-2 text-xs font-medium text-sky-900">
          影响：{item.impact.join("、")}
        </div>
      ) : null}
      {item.action ? (
        <div className="mt-2 rounded-xl border border-border bg-muted/60 px-3 py-2 text-xs leading-5 text-muted-foreground">
          建议：{item.action}
        </div>
      ) : null}
    </article>
  );
}
