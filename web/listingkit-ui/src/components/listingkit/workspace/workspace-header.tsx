"use client";

import Link from "next/link";

import { RecoverySummaryCard } from "@/components/listingkit/review/recovery-summary-card";
import { ResolvedActionCard } from "@/components/listingkit/review/resolved-action-card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type {
  RecoveryDescriptor,
  RecoverySummary,
  ResolvedActionSummary,
} from "@/lib/types/listingkit";

export function WorkspaceHeader({
  title,
  subtitle,
  statusLabel,
  updatedAtLabel,
  summary,
  recoverySummary,
  showSheinStudioLink = false,
  showLayerActions = false,
  onRunStandardLayer,
  onRunPlatformLayer,
  layerActionsPending = false,
  onSelectAction,
  onSelectRecovery,
}: {
  title: string;
  subtitle?: string;
  statusLabel?: string;
  updatedAtLabel?: string;
  summary?: ResolvedActionSummary | null;
  recoverySummary?: RecoverySummary | null;
  showSheinStudioLink?: boolean;
  showLayerActions?: boolean;
  onRunStandardLayer?: () => void;
  onRunPlatformLayer?: () => void;
  layerActionsPending?: boolean;
  onSelectAction?: (summary: ResolvedActionSummary) => void;
  onSelectRecovery?: (descriptor: RecoveryDescriptor) => void;
}) {
  return (
    <section className="grid min-w-0 gap-4 2xl:grid-cols-[minmax(0,2fr)_minmax(20rem,1fr)]">
      <div className="min-w-0 space-y-3">
        <div className="min-w-0">
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-muted-foreground">
            ListingKit 工作台
          </p>
          <div className="mt-3 flex flex-wrap items-center gap-2 text-sm">
            <Link
              href="/listing-kits"
              className="inline-flex items-center rounded-full border border-border bg-background px-3 py-1.5 font-medium text-muted-foreground transition hover:border-ring hover:text-foreground"
            >
              返回任务列表
            </Link>
            {showSheinStudioLink ? (
              <Link
                href="/listing-kits/sds"
                className="inline-flex items-center rounded-full border border-border bg-background px-3 py-1.5 font-medium text-muted-foreground transition hover:border-ring hover:text-foreground"
              >
                返回 POD 工作室
              </Link>
            ) : null}
            {showLayerActions ? (
              <details className="rounded-full border border-border bg-background px-3 py-1.5">
                <summary className="cursor-pointer list-none text-sm font-medium text-muted-foreground">
                  高级操作
                </summary>
                <div className="mt-3 flex flex-wrap gap-2">
                  <Button
                    onClick={onRunStandardLayer}
                    type="button"
                    variant="secondary"
                    disabled={layerActionsPending}
                  >
                    运行标准商品层
                  </Button>
                  <Button
                    onClick={onRunPlatformLayer}
                    type="button"
                    variant="secondary"
                    disabled={layerActionsPending}
                  >
                    运行平台适配层
                  </Button>
                </div>
              </details>
            ) : null}
          </div>
          <h1 className="mt-2 break-words text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">
            {title}
          </h1>
          {statusLabel || updatedAtLabel ? (
            <div className="mt-3 flex flex-wrap gap-2 text-xs text-muted-foreground">
              {statusLabel ? (
                <Badge className="gap-2 rounded-full px-3 py-1.5" variant="neutral">
                  <span className="font-semibold text-foreground">任务状态</span>
                  <span>{statusLabel}</span>
                </Badge>
              ) : null}
              {updatedAtLabel ? (
                <Badge className="gap-2 rounded-full px-3 py-1.5" variant="neutral">
                  <span className="font-semibold text-foreground">最近更新</span>
                  <span>{updatedAtLabel}</span>
                </Badge>
              ) : null}
            </div>
          ) : null}
          {subtitle ? (
            <p className="mt-2 break-all text-xs text-muted-foreground">
              {subtitle}
            </p>
          ) : null}
        </div>
        <ResolvedActionCard summary={summary} onSelect={onSelectAction} />
      </div>
      <RecoverySummaryCard summary={recoverySummary} onSelect={onSelectRecovery} />
    </section>
  );
}
