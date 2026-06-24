"use client";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
  listListingDispatchEvents,
  type DispatchEventItem,
  type DispatchEventQuery,
  type DispatchEventSummary,
} from "@/lib/api/admin-dispatch-events";
import { useQuery } from "@tanstack/react-query";
import { Activity, ChevronLeft, ChevronRight, RefreshCw, Search } from "lucide-react";
import { FormEvent, useMemo, useState } from "react";

const DEFAULT_PAGE_SIZE = 50;

const ACTION_TEXT: Record<string, string> = {
  dispatched: "已派发",
  skipped: "已跳过",
  failed: "失败",
};

const ACTION_BADGE_VARIANT: Record<string, "default" | "neutral" | "destructive"> = {
  dispatched: "default",
  skipped: "neutral",
  failed: "destructive",
};

type DispatchEventFilters = {
  platform: string;
  tenantId: string;
  storeId: string;
  action: string;
  reasonCode: string;
  from: string;
  to: string;
};

const EMPTY_FILTERS: DispatchEventFilters = {
  platform: "shein",
  tenantId: "",
  storeId: "",
  action: "",
  reasonCode: "",
  from: "",
  to: "",
};

export function DispatchEventsAdminPage() {
  const [filters, setFilters] = useState<DispatchEventFilters>(EMPTY_FILTERS);
  const [submittedFilters, setSubmittedFilters] =
    useState<DispatchEventFilters>(EMPTY_FILTERS);
  const [page, setPage] = useState(1);

  const query = useMemo<DispatchEventQuery>(
    () => ({
      platform: submittedFilters.platform,
      tenantId: submittedFilters.tenantId,
      storeId: submittedFilters.storeId,
      action: submittedFilters.action,
      reasonCode: submittedFilters.reasonCode,
      from: toRFC3339(submittedFilters.from),
      to: toRFC3339(submittedFilters.to),
      page,
      page_size: DEFAULT_PAGE_SIZE,
    }),
    [page, submittedFilters],
  );

  const summaryQuery = useQuery({
    queryKey: ["listingkit-admin-dispatch-event-summary", query],
    queryFn: () => getListingDispatchEventSummary(query),
    refetchInterval: 30_000,
  });

  const eventsQuery = useQuery({
    queryKey: ["listingkit-admin-dispatch-events", query],
    queryFn: () => listListingDispatchEvents(query),
    refetchInterval: 30_000,
  });

  const summary = summaryQuery.data;
  const pageData = eventsQuery.data;
  const items = pageData?.items ?? [];
  const initialLoading = summaryQuery.isLoading || eventsQuery.isLoading;
  const refreshing =
    summaryQuery.isLoading ||
    summaryQuery.isFetching ||
    eventsQuery.isLoading ||
    eventsQuery.isFetching;
  const visibleError = firstErrorMessage(summaryQuery.error, eventsQuery.error);
  const totalPages = Math.max(
    1,
    Math.ceil((pageData?.total ?? 0) / (pageData?.pageSize ?? DEFAULT_PAGE_SIZE)),
  );

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPage(1);
    setSubmittedFilters(filters);
  }

  function updateFilter(key: keyof DispatchEventFilters, value: string) {
    setFilters((current) => ({ ...current, [key]: value }));
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">调度事件</h1>
            <p className="mt-1 text-sm text-zinc-500">
              观察 ListingKit 调度结果、跳过原因和店铺容量阻塞。默认查询最近 60 分钟。
            </p>
          </div>
          <Button
            type="button"
            className="w-full xl:w-auto"
            variant="secondary"
            onClick={() => {
              void summaryQuery.refetch();
              void eventsQuery.refetch();
            }}
          >
            {refreshing ? (
              <RefreshCw className="size-4 animate-spin" />
            ) : (
              <Search className="size-4" />
            )}
            刷新
          </Button>
        </div>
        <form
          className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-4"
          onSubmit={handleSubmit}
        >
          <FilterInput
            label="平台"
            value={filters.platform}
            onChange={(value) => updateFilter("platform", value)}
            placeholder="shein"
          />
          <FilterInput
            label="租户 ID"
            value={filters.tenantId}
            onChange={(value) => updateFilter("tenantId", value)}
            placeholder="默认当前租户"
          />
          <FilterInput
            label="店铺 ID"
            value={filters.storeId}
            onChange={(value) => updateFilter("storeId", value)}
            placeholder="976"
          />
          <FilterInput
            label="动作"
            value={filters.action}
            onChange={(value) => updateFilter("action", value)}
            placeholder="dispatched / skipped / failed"
          />
          <FilterInput
            label="原因"
            value={filters.reasonCode}
            onChange={(value) => updateFilter("reasonCode", value)}
            placeholder="no_capacity"
          />
          <FilterInput
            label="开始时间"
            type="datetime-local"
            value={filters.from}
            onChange={(value) => updateFilter("from", value)}
          />
          <FilterInput
            label="结束时间"
            type="datetime-local"
            value={filters.to}
            onChange={(value) => updateFilter("to", value)}
          />
          <div className="flex items-end gap-2">
            <Button type="submit" className="w-full">
              <Search className="size-4" />
              查询
            </Button>
          </div>
        </form>
        {visibleError ? (
          <Alert className="mt-4" variant="destructive">
            <AlertDescription>{visibleError}</AlertDescription>
          </Alert>
        ) : null}
      </section>

      <SummaryCards summary={summary} />

      <section className="grid gap-4 xl:grid-cols-[1fr_1.3fr]">
        <ReasonDistribution summary={summary} />
        <StoreBlockers summary={summary} />
      </section>

      <section className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
        <div className="flex flex-col gap-2 border-b border-zinc-200 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 className="text-base font-semibold text-zinc-950">事件明细</h2>
            <p className="text-sm text-zinc-500">
              共 {pageData?.total ?? 0} 条，当前第 {page} / {totalPages} 页
            </p>
          </div>
          <div className="flex gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={page <= 1 || initialLoading}
              onClick={() => setPage((current) => Math.max(1, current - 1))}
            >
              <ChevronLeft className="size-4" />
              上一页
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={page >= totalPages || initialLoading}
              onClick={() => setPage((current) => current + 1)}
            >
              下一页
              <ChevronRight className="size-4" />
            </Button>
          </div>
        </div>
        <DispatchEventTable items={items} loading={initialLoading} />
      </section>
    </div>
  );
}

function SummaryCards({ summary }: { summary?: DispatchEventSummary }) {
  return (
    <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <StatTile label="事件总数" value={summary?.total ?? 0} />
      <StatTile label="已派发" value={summary?.dispatched ?? 0} />
      <StatTile label="已跳过" value={summary?.skipped ?? 0} />
      <StatTile label="失败" value={summary?.failed ?? 0} />
    </section>
  );
}

function ReasonDistribution({ summary }: { summary?: DispatchEventSummary }) {
  const rows = summary?.reasonCounts ?? [];
  return (
    <section className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
      <h2 className="text-base font-semibold text-zinc-950">原因分布</h2>
      <div className="mt-3 space-y-2">
        {rows.length === 0 ? (
          <p className="text-sm text-zinc-500">暂无原因数据</p>
        ) : (
          rows.slice(0, 10).map((row) => (
            <div
              key={`${row.action}:${row.reasonCode}`}
              className="flex items-center justify-between rounded-md bg-zinc-50 px-3 py-2"
            >
              <div className="min-w-0">
                <div className="truncate font-mono text-sm text-zinc-950">
                  {row.reasonCode || "-"}
                </div>
                <div className="text-xs text-zinc-500">
                  {ACTION_TEXT[row.action] ?? row.action}
                </div>
              </div>
              <Badge className="rounded-full px-2 py-1" variant="neutral">
                {row.count}
              </Badge>
            </div>
          ))
        )}
      </div>
    </section>
  );
}

function StoreBlockers({ summary }: { summary?: DispatchEventSummary }) {
  const rows = summary?.storeBlockers ?? [];
  return (
    <section className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
      <h2 className="text-base font-semibold text-zinc-950">店铺阻塞 Top</h2>
      <div className="mt-3 overflow-x-auto">
        <Table className="min-w-[44rem]">
          <TableHeader className="bg-zinc-50">
            <TableRow className="hover:bg-transparent">
              <TableHead>店铺</TableHead>
              <TableHead>原因</TableHead>
              <TableHead>次数</TableHead>
              <TableHead>容量</TableHead>
              <TableHead>节点</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 ? (
              <TableRow>
                <TableCell className="py-6 text-zinc-500" colSpan={5}>
                  暂无阻塞数据
                </TableCell>
              </TableRow>
            ) : (
              rows.map((row) => (
                <TableRow key={`${row.tenantId}:${row.storeId}:${row.reasonCode}`}>
                  <TableCell>
                    <div className="font-mono text-sm text-zinc-950">
                      {row.storeId}
                    </div>
                    <div className="text-xs text-zinc-500">租户 {row.tenantId}</div>
                  </TableCell>
                  <TableCell className="font-mono text-sm">
                    {row.reasonCode}
                  </TableCell>
                  <TableCell>{row.count}</TableCell>
                  <TableCell className="text-sm text-zinc-700">
                    队列 {row.maxQueued}，处理中 {row.maxProcessing}，今日完成{" "}
                    {row.maxCompletedToday} / {row.dailyLimit}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-zinc-500">
                    {row.ownerNode || "-"}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </section>
  );
}

function DispatchEventTable({
  items,
  loading,
}: {
  items: DispatchEventItem[];
  loading: boolean;
}) {
  return (
    <div className="overflow-x-auto">
      <Table className="min-w-[72rem]">
        <TableHeader className="bg-zinc-50">
          <TableRow className="text-xs uppercase tracking-[0.2em] hover:bg-transparent">
            <TableHead>时间</TableHead>
            <TableHead>任务</TableHead>
            <TableHead>店铺</TableHead>
            <TableHead>动作</TableHead>
            <TableHead>原因</TableHead>
            <TableHead>容量快照</TableHead>
            <TableHead>节点</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {loading && items.length === 0 ? (
            <TableRow>
              <TableCell className="py-6 text-zinc-500" colSpan={7}>
                加载中...
              </TableCell>
            </TableRow>
          ) : items.length === 0 ? (
            <TableRow>
              <TableCell className="py-6 text-zinc-500" colSpan={7}>
                暂无调度事件
              </TableCell>
            </TableRow>
          ) : (
            items.map((item) => (
              <TableRow key={item.id} className="align-top">
                <TableCell className="whitespace-nowrap text-zinc-700">
                  {formatDateTime(item.createdAt)}
                </TableCell>
                <TableCell>
                  <div className="font-mono text-sm text-zinc-950">
                    #{item.taskId}
                  </div>
                  <div className="text-xs text-zinc-500">
                    {item.stage || "dispatch"}
                  </div>
                </TableCell>
                <TableCell>
                  <div className="font-mono text-sm text-zinc-950">
                    {item.storeId}
                  </div>
                  <div className="text-xs text-zinc-500">
                    租户 {item.tenantId} · {item.platform || "-"}
                  </div>
                </TableCell>
                <TableCell>
                  <Badge
                    className="rounded-full px-2 py-1 text-xs"
                    variant={ACTION_BADGE_VARIANT[item.action] ?? "neutral"}
                  >
                    {ACTION_TEXT[item.action] ?? item.action}
                  </Badge>
                </TableCell>
                <TableCell className="font-mono text-sm text-zinc-700">
                  {item.reasonCode || "-"}
                </TableCell>
                <TableCell className="text-sm text-zinc-700">
                  <div>可派发 {item.capacity}</div>
                  <div className="text-xs text-zinc-500">
                    队列 {item.queued}，处理中 {item.processing}，今日完成{" "}
                    {item.completedToday} / {item.dailyLimit}
                  </div>
                </TableCell>
                <TableCell className="font-mono text-xs text-zinc-500">
                  {item.ownerNode || "-"}
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </div>
  );
}

function FilterInput({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  type?: string;
}) {
  return (
    <Label className="block text-xs font-medium text-zinc-500">
      {label}
      <Input
        className="mt-1 h-9"
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
      />
    </Label>
  );
}

function StatTile({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
      <div className="mb-2 flex items-center gap-2 text-xs font-medium text-zinc-500">
        <Activity className="size-4" />
        {label}
      </div>
      <div className="text-2xl font-semibold text-zinc-950">{value}</div>
    </div>
  );
}

function toRFC3339(value: string) {
  if (!value) {
    return undefined;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return undefined;
  }
  return date.toISOString();
}

function formatDateTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function firstErrorMessage(...errors: unknown[]) {
  for (const error of errors) {
    if (error instanceof Error) {
      return error.message;
    }
  }
  return "";
}
