"use client";

import { useQuery } from "@tanstack/react-query";
import { Activity, BarChart3, RefreshCw, Search } from "lucide-react";
import { FormEvent, useMemo, useState, type ReactNode } from "react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  getListingDispatchEventSummary,
  getListingDispatchEvents,
  type DispatchEventQuery,
} from "@/lib/api/admin-dispatch-events";
import { formatSubscriptionApiError } from "@/lib/api/subscription";

type FilterState = {
  platform: string;
  tenantId: string;
  storeId: string;
  action: string;
  reasonCode: string;
  from: string;
  to: string;
};

const DEFAULT_FILTERS: FilterState = {
  platform: "",
  tenantId: "",
  storeId: "",
  action: "",
  reasonCode: "",
  from: "",
  to: "",
};

const ACTION_OPTIONS = ["", "dispatched", "skipped", "failed"] as const;
const PAGE_SIZE_OPTIONS = [20, 50, 100, 200] as const;

export function DispatchEventsAdminPage() {
  const [filters, setFilters] = useState<FilterState>(DEFAULT_FILTERS);
  const [appliedFilters, setAppliedFilters] =
    useState<FilterState>(DEFAULT_FILTERS);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(50);

  const summaryQueryParams = useMemo(
    () => buildDispatchEventQuery(appliedFilters),
    [appliedFilters],
  );
  const eventQueryParams = useMemo<DispatchEventQuery>(
    () => ({
      ...summaryQueryParams,
      page,
      page_size: pageSize,
    }),
    [page, pageSize, summaryQueryParams],
  );

  const summaryQuery = useQuery({
    queryKey: ["listingkit-admin-dispatch-event-summary", summaryQueryParams],
    queryFn: () => getListingDispatchEventSummary(summaryQueryParams),
    refetchInterval: 30_000,
  });
  const eventsQuery = useQuery({
    queryKey: ["listingkit-admin-dispatch-events", eventQueryParams],
    queryFn: () => getListingDispatchEvents(eventQueryParams),
    refetchInterval: 30_000,
  });

  const summary = summaryQuery.data;
  const eventPage = eventsQuery.data;
  const events = eventPage?.items ?? [];
  const total = eventPage?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const totalSummaryEvents = summary?.total ?? 0;
  const dispatchedPercent = percentage(summary?.dispatched ?? 0, totalSummaryEvents);
  const skippedPercent = percentage(summary?.skipped ?? 0, totalSummaryEvents);
  const failedPercent = percentage(summary?.failed ?? 0, totalSummaryEvents);
  const failedReasons =
    summary?.reasonCounts.filter((item) => item.action === "failed") ?? [];
  const skippedReasons =
    summary?.reasonCounts.filter((item) => item.action === "skipped") ?? [];
  const topFailedReason = failedReasons[0];
  const topSkippedReason = skippedReasons[0];
  const loading =
    summaryQuery.isLoading ||
    summaryQuery.isFetching ||
    eventsQuery.isLoading ||
    eventsQuery.isFetching;
  const visibleError =
    summaryQuery.error instanceof Error
      ? formatSubscriptionApiError(summaryQuery.error)
      : eventsQuery.error instanceof Error
        ? formatSubscriptionApiError(eventsQuery.error)
        : "";

  function updateFilter(key: keyof FilterState, value: string) {
    setFilters((current) => ({ ...current, [key]: value }));
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPage(1);
    setAppliedFilters({ ...filters });
    if (filtersMatch(filters, appliedFilters)) {
      void Promise.all([summaryQuery.refetch(), eventsQuery.refetch()]);
    }
  }

  function handlePageSizeChange(value: string) {
    setPage(1);
    setPageSize(Number(value));
  }

  function applyQuickAction(action: FilterState["action"]) {
    const nextFilters = { ...appliedFilters, action };
    setFilters(nextFilters);
    setAppliedFilters(nextFilters);
    setPage(1);
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between">
          <div>
            <div className="mb-2 flex items-center gap-2">
              <Activity className="size-5 text-zinc-500" />
              <h1 className="text-2xl font-semibold text-zinc-950">
                调度事件
              </h1>
            </div>
            <p className="text-sm text-zinc-500">
              查看 ListingKit dispatch、skipped、failed 决策。未选择时间时由后端使用默认 60 分钟窗口。
            </p>
          </div>
          <form
            className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4 xl:max-w-5xl xl:grid-cols-8"
            onSubmit={handleSubmit}
          >
            <DispatchEventInput
              label="平台"
              value={filters.platform}
              onChange={(value) => updateFilter("platform", value)}
              placeholder="shein"
            />
            <DispatchEventInput
              label="租户 ID"
              value={filters.tenantId}
              onChange={(value) => updateFilter("tenantId", value)}
              type="number"
            />
            <DispatchEventInput
              label="店铺 ID"
              value={filters.storeId}
              onChange={(value) => updateFilter("storeId", value)}
              type="number"
            />
            <DispatchEventSelect
              label="动作"
              value={filters.action}
              onChange={(value) => updateFilter("action", value)}
              options={ACTION_OPTIONS}
              labels={{ "": "全部" }}
            />
            <DispatchEventInput
              label="原因"
              value={filters.reasonCode}
              onChange={(value) => updateFilter("reasonCode", value)}
              placeholder="no_capacity"
            />
            <DispatchEventInput
              label="开始时间"
              value={filters.from}
              onChange={(value) => updateFilter("from", value)}
              type="datetime-local"
            />
            <DispatchEventInput
              label="结束时间"
              value={filters.to}
              onChange={(value) => updateFilter("to", value)}
              type="datetime-local"
            />
            <Button
              type="submit"
              className="w-full sm:mt-5"
              variant="secondary"
            >
              {loading ? (
                <RefreshCw className="size-4 animate-spin" />
              ) : (
                <Search className="size-4" />
              )}
              查询
            </Button>
          </form>
        </div>
        {visibleError ? (
          <Alert className="mt-4" variant="destructive">
            <AlertDescription>{visibleError}</AlertDescription>
          </Alert>
        ) : null}
      </section>

      {summary && summary.failed > 0 ? (
        <Alert variant="destructive">
          <AlertDescription>
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                最近窗口出现 {summary.failed} 次 failed 调度事件
                {topFailedReason
                  ? `，最高频原因是 ${topFailedReason.reasonCode}（${topFailedReason.count} 次）`
                  : ""}
                。建议优先查看失败事件和后端日志。
              </div>
              <Button
                type="button"
                variant="secondary"
                onClick={() => applyQuickAction("failed")}
              >
                只看失败
              </Button>
            </div>
          </AlertDescription>
        </Alert>
      ) : null}

      <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          label="总事件"
          value={totalSummaryEvents}
          detail={formatWindow(summary?.window.from, summary?.window.to)}
        />
        <MetricCard
          label="已派发"
          value={summary?.dispatched ?? 0}
          detail={`${dispatchedPercent}% of window`}
          tone="good"
        />
        <MetricCard
          label="已跳过"
          value={summary?.skipped ?? 0}
          detail={
            topSkippedReason
              ? `${skippedPercent}% · ${topSkippedReason.reasonCode}`
              : `${skippedPercent}% of window`
          }
          tone={summary && summary.skipped > 0 ? "warn" : "neutral"}
        />
        <MetricCard
          label="失败"
          value={summary?.failed ?? 0}
          detail={
            topFailedReason
              ? `${failedPercent}% · ${topFailedReason.reasonCode}`
              : `${failedPercent}% of window`
          }
          tone={summary && summary.failed > 0 ? "danger" : "neutral"}
        />
      </section>

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(360px,0.75fr)]">
        <Panel
          title="原因分布"
          description={formatWindow(summary?.window.from, summary?.window.to)}
        >
          {loading && !summary ? (
            <EmptyState>加载中...</EmptyState>
          ) : (summary?.reasonCounts ?? []).length === 0 ? (
            <EmptyState>暂无原因分布</EmptyState>
          ) : (
            <div className="space-y-2">
              {summary?.reasonCounts.map((item) => {
                const percent = summary.total
                  ? Math.round((item.count / summary.total) * 100)
                  : 0;
                return (
                  <div
                    key={`${item.action}:${item.reasonCode}`}
                    className="rounded-md border border-zinc-100 p-3"
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <Badge
                            className="rounded-full px-2 py-1 text-xs"
                            variant="secondary"
                          >
                            {displayText(item.reasonCode, "<empty>")}
                          </Badge>
                          <span className="font-mono text-xs text-zinc-500">
                            {item.action}
                          </span>
                        </div>
                      </div>
                      <div className="text-right">
                        <div className="font-semibold text-zinc-950">
                          {item.count}
                        </div>
                        <div className="text-xs text-zinc-500">{percent}%</div>
                      </div>
                    </div>
                    <div className="mt-2 h-2 rounded-full bg-zinc-100">
                      <div
                        className="h-2 rounded-full bg-zinc-950"
                        style={{ width: `${Math.min(100, percent)}%` }}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </Panel>

        <Panel title="店铺阻塞 Top" description="按 skipped 事件聚合">
          {loading && !summary ? (
            <EmptyState>加载中...</EmptyState>
          ) : (summary?.storeBlockers ?? []).length === 0 ? (
            <EmptyState>暂无阻塞店铺</EmptyState>
          ) : (
            <div className="space-y-2">
              {summary?.storeBlockers.map((item) => (
                <div
                  key={`${item.tenantId}:${item.storeId}:${item.reasonCode}`}
                  className="rounded-md border border-zinc-100 p-3"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="font-medium text-zinc-950">
                        租户 {item.tenantId} / 店铺 {item.storeId}
                      </div>
                      <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-zinc-500">
                        <Badge
                          className="rounded-full px-2 py-1 text-xs"
                          variant="neutral"
                        >
                          {displayText(item.reasonCode, "<empty>")}
                        </Badge>
                        <span>{displayText(item.ownerNode, "-")}</span>
                      </div>
                    </div>
                    <div className="text-right">
                      <div className="font-semibold text-zinc-950">
                        {item.count}
                      </div>
                      <div className="text-xs text-zinc-500">次</div>
                    </div>
                  </div>
                  <div className="mt-3 grid grid-cols-2 gap-2 text-xs text-zinc-600 sm:grid-cols-4">
                    <BlockerMetric label="日限" value={item.dailyLimit} />
                    <BlockerMetric label="队列峰值" value={item.maxQueued} />
                    <BlockerMetric label="处理中" value={item.maxProcessing} />
                    <BlockerMetric
                      label="今日完成"
                      value={item.maxCompletedToday}
                    />
                  </div>
                </div>
              ))}
            </div>
          )}
        </Panel>
      </section>

      <section className="grid gap-4 xl:grid-cols-2">
        <Panel
          title="失败原因 Top"
          description="failed > 0 时优先排查这里"
          action={
            <Button
              type="button"
              variant="outline"
              onClick={() => applyQuickAction("failed")}
              disabled={loading}
            >
              筛选失败
            </Button>
          }
        >
          {loading && !summary ? (
            <EmptyState>加载中...</EmptyState>
          ) : failedReasons.length === 0 ? (
            <EmptyState>当前窗口暂无 failed 事件</EmptyState>
          ) : (
            <ReasonList items={failedReasons} total={summary?.failed ?? 0} />
          )}
        </Panel>

        <Panel
          title="跳过原因 Top"
          description="用于解释 pending 任务为什么暂未调度"
          action={
            <Button
              type="button"
              variant="outline"
              onClick={() => applyQuickAction("skipped")}
              disabled={loading}
            >
              筛选跳过
            </Button>
          }
        >
          {loading && !summary ? (
            <EmptyState>加载中...</EmptyState>
          ) : skippedReasons.length === 0 ? (
            <EmptyState>当前窗口暂无 skipped 事件</EmptyState>
          ) : (
            <ReasonList items={skippedReasons} total={summary?.skipped ?? 0} />
          )}
        </Panel>
      </section>

      <section className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
        <div className="flex flex-col gap-3 border-b border-zinc-100 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 className="text-base font-semibold text-zinc-950">
              最近事件
            </h2>
            <p className="text-sm text-zinc-500">
              共 {total} 条，当前第 {page} / {totalPages} 页。
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Label className="text-xs font-medium text-zinc-500">
              每页
              <Select
                value={String(pageSize)}
                onChange={(event) => handlePageSizeChange(event.target.value)}
                className="ml-2 h-9 rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900"
              >
                {PAGE_SIZE_OPTIONS.map((option) => (
                  <option key={option} value={option}>
                    {option}
                  </option>
                ))}
              </Select>
            </Label>
            <Button
              type="button"
              variant="outline"
              disabled={page <= 1 || loading}
              onClick={() => setPage((current) => Math.max(1, current - 1))}
            >
              上一页
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={page >= totalPages || loading}
              onClick={() =>
                setPage((current) => Math.min(totalPages, current + 1))
              }
            >
              下一页
            </Button>
          </div>
        </div>
        <div className="overflow-x-auto">
          <Table className="min-w-[72rem] divide-y divide-zinc-200 text-sm">
            <TableHeader className="bg-zinc-50 text-left text-xs font-semibold uppercase text-zinc-500">
              <TableRow>
                <TableHead className="px-4 py-3">时间</TableHead>
                <TableHead className="px-4 py-3">任务</TableHead>
                <TableHead className="px-4 py-3">租户 / 店铺</TableHead>
                <TableHead className="px-4 py-3">平台</TableHead>
                <TableHead className="px-4 py-3">动作</TableHead>
                <TableHead className="px-4 py-3">原因 / 阶段</TableHead>
                <TableHead className="px-4 py-3">容量</TableHead>
                <TableHead className="px-4 py-3">队列</TableHead>
                <TableHead className="px-4 py-3">日限</TableHead>
                <TableHead className="px-4 py-3">Owner</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody className="divide-y divide-zinc-100">
              {loading && events.length === 0 ? (
                <TableRow>
                  <TableCell className="px-4 py-6 text-zinc-500" colSpan={10}>
                    加载中...
                  </TableCell>
                </TableRow>
              ) : events.length === 0 ? (
                <TableRow>
                  <TableCell className="px-4 py-6 text-zinc-500" colSpan={10}>
                    暂无调度事件
                  </TableCell>
                </TableRow>
              ) : (
                events.map((item) => (
                  <TableRow key={item.id} className="align-top">
                    <TableCell className="px-4 py-3 text-zinc-600">
                      {formatTime(item.createdAt)}
                    </TableCell>
                    <TableCell className="px-4 py-3">
                      <div className="font-mono text-zinc-800">
                        {item.taskId}
                      </div>
                      <div className="font-mono text-xs text-zinc-500">
                        #{item.id}
                      </div>
                    </TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">
                      {item.tenantId} / {item.storeId}
                    </TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">
                      {displayText(item.platform, "-")}
                    </TableCell>
                    <TableCell className="px-4 py-3">
                      <Badge
                        className="rounded-full px-2 py-1 text-xs"
                        variant={
                          item.action === "dispatched"
                            ? "secondary"
                            : "neutral"
                        }
                      >
                        {item.action}
                      </Badge>
                    </TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">
                      <div>{displayText(item.reasonCode, "-")}</div>
                      <div className="font-mono text-xs text-zinc-500">
                        {displayText(item.stage, "-")}
                      </div>
                    </TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">
                      {item.capacity}
                    </TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">
                      <div>queued {item.queued}</div>
                      <div className="text-xs text-zinc-500">
                        processing {item.processing}
                      </div>
                    </TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">
                      <div>{item.dailyLimit}</div>
                      <div className="text-xs text-zinc-500">
                        done {item.completedToday}
                      </div>
                    </TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">
                      {displayText(item.ownerNode, "-")}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </section>
    </div>
  );
}

function buildDispatchEventQuery(filters: FilterState): DispatchEventQuery {
  return compactQuery({
    platform: trimOrUndefined(filters.platform),
    tenantId: numberOrUndefined(filters.tenantId),
    storeId: numberOrUndefined(filters.storeId),
    action: trimOrUndefined(filters.action),
    reasonCode: trimOrUndefined(filters.reasonCode),
    from: dateTimeOrUndefined(filters.from),
    to: dateTimeOrUndefined(filters.to),
  });
}

function compactQuery(query: DispatchEventQuery): DispatchEventQuery {
  return Object.fromEntries(
    Object.entries(query).filter(([, value]) => value !== undefined),
  ) as DispatchEventQuery;
}

function trimOrUndefined(value: string) {
  const trimmed = value.trim();
  return trimmed || undefined;
}

function numberOrUndefined(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return undefined;
  }
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : undefined;
}

function dateTimeOrUndefined(value: string) {
  if (!value) {
    return undefined;
  }
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? undefined : parsed.toISOString();
}

function filtersMatch(left: FilterState, right: FilterState) {
  return (Object.keys(DEFAULT_FILTERS) as Array<keyof FilterState>).every(
    (key) => left[key] === right[key],
  );
}

function formatWindow(from: string | undefined, to: string | undefined) {
  if (!from || !to) {
    return "等待后端返回统计窗口";
  }
  return `${formatTime(from)} - ${formatTime(to)}`;
}

function formatTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", {
    hour12: false,
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function percentage(value: number, total: number) {
  if (total <= 0) {
    return 0;
  }
  return Math.round((value / total) * 100);
}

function displayText(value: string | undefined, fallback: string) {
  return value?.trim() || fallback;
}

function MetricCard({
  label,
  value,
  detail,
  tone = "neutral",
}: {
  label: string;
  value: number;
  detail?: string;
  tone?: "neutral" | "good" | "warn" | "danger";
}) {
  const toneClass =
    tone === "good"
      ? "border-emerald-200 bg-emerald-50 text-emerald-950"
      : tone === "warn"
        ? "border-amber-200 bg-amber-50 text-amber-950"
        : tone === "danger"
          ? "border-rose-200 bg-rose-50 text-rose-950"
          : "border-zinc-200 bg-white text-zinc-950";
  return (
    <div className={`rounded-lg border p-4 shadow-sm ${toneClass}`}>
      <div className="mb-2 flex items-center gap-2 text-xs font-medium opacity-70">
        <BarChart3 className="size-4" />
        {label}
      </div>
      <div className="text-2xl font-semibold">{value}</div>
      {detail ? <div className="mt-1 text-xs opacity-70">{detail}</div> : null}
    </div>
  );
}

function Panel({
  title,
  description,
  action,
  children,
}: {
  title: string;
  description: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
      <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 className="text-base font-semibold text-zinc-950">{title}</h2>
          <p className="mt-1 text-xs text-zinc-500">{description}</p>
        </div>
        {action ? <div className="shrink-0">{action}</div> : null}
      </div>
      {children}
    </div>
  );
}

function ReasonList({
  items,
  total,
}: {
  items: Array<{ reasonCode: string; action: string; count: number }>;
  total: number;
}) {
  return (
    <div className="space-y-2">
      {items.map((item) => {
        const percent = percentage(item.count, total);
        return (
          <div
            key={`${item.action}:${item.reasonCode}`}
            className="rounded-md border border-zinc-100 p-3"
          >
            <div className="flex items-center justify-between gap-3">
              <Badge className="rounded-full px-2 py-1 text-xs" variant="neutral">
                {displayText(item.reasonCode, "<empty>")}
              </Badge>
              <div className="text-right">
                <div className="font-semibold text-zinc-950">{item.count}</div>
                <div className="text-xs text-zinc-500">{percent}%</div>
              </div>
            </div>
            <div className="mt-2 h-2 rounded-full bg-zinc-100">
              <div
                className="h-2 rounded-full bg-zinc-950"
                style={{ width: `${Math.min(100, percent)}%` }}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}

function EmptyState({ children }: { children: ReactNode }) {
  return <div className="py-6 text-sm text-zinc-500">{children}</div>;
}

function BlockerMetric({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-md bg-zinc-50 px-2 py-1.5">
      <div className="text-zinc-500">{label}</div>
      <div className="font-semibold text-zinc-900">{value}</div>
    </div>
  );
}

function DispatchEventInput({
  label,
  value,
  onChange,
  type = "text",
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  type?: string;
  placeholder?: string;
}) {
  return (
    <Label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <Input
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
      />
    </Label>
  );
}

function DispatchEventSelect({
  label,
  value,
  onChange,
  options,
  labels = {},
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: readonly string[];
  labels?: Record<string, string>;
}) {
  return (
    <Label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <Select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900"
      >
        {options.map((option) => (
          <option key={option} value={option}>
            {labels[option] ?? option}
          </option>
        ))}
      </Select>
    </Label>
  );
}
