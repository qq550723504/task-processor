"use client";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { useQuery } from "@tanstack/react-query";
import { BarChart3, RefreshCw, Search } from "lucide-react";
import { FormEvent, useEffect, useMemo, useState } from "react";

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
  getListingStoreStatistics,
  type ListingStoreStatistics,
  type ListingStoreStatisticsPage,
  type ListingStoreStatisticsSummary,
} from "@/lib/api/admin-store-statistics";

const TODAY = new Date().toISOString().slice(0, 10);
const DEFAULT_PAGE = 1;
const DEFAULT_PAGE_SIZE = 20;
const PAGE_SIZE_OPTIONS = [20, 50, 100] as const;

const STATUS_TEXT: Record<number, string> = {
  0: "启用",
  1: "禁用",
};

const LIMIT_TYPE_TEXT: Record<string, string> = {
  fixed: "固定",
  dynamic: "动态",
};

type StoreStatisticsPageVariant = "admin" | "tenant";

type StoreStatisticsAdminPageProps = {
  variant?: StoreStatisticsPageVariant;
};

export function StoreStatisticsAdminPage({
  variant = "admin",
}: StoreStatisticsAdminPageProps = {}) {
  const [date, setDate] = useState(TODAY);
  const [page, setPage] = useState(DEFAULT_PAGE);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
  const isTenantView = variant === "tenant";

  const query = useMemo(
    () => ({
      date,
      page,
      page_size: pageSize,
    }),
    [date, page, pageSize],
  );

  const statisticsQuery = useQuery({
    queryKey: ["listingkit-admin-store-statistics", query],
    queryFn: () => getListingStoreStatistics(query),
    refetchInterval: 30_000,
    placeholderData: (previousData) => previousData,
  });

  const pageData = statisticsQuery.data;
  const items = pageData?.items ?? [];
  const summary = pageData?.summary ?? emptySummary();
  const total = pageData?.total ?? 0;
  const loading = statisticsQuery.isLoading || statisticsQuery.isFetching;
  const visibleError =
    statisticsQuery.error instanceof Error ? statisticsQuery.error.message : "";
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const currentPage = Math.min(Math.max(page, 1), totalPages);

  useEffect(() => {
    if (!pageData) {
      return;
    }
    const lastValidPage = Math.max(1, Math.ceil(pageData.total / pageSize));
    if (page > lastValidPage) {
      setPage(lastValidPage);
    }
  }, [page, pageData, pageSize]);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void statisticsQuery.refetch();
  }

  function handleDateChange(nextDate: string) {
    setDate(nextDate);
    setPage(DEFAULT_PAGE);
  }

  function handlePageSizeChange(nextPageSize: number) {
    setPageSize(nextPageSize);
    setPage(DEFAULT_PAGE);
  }

  function handlePageChange(nextPage: number) {
    setPage(nextPage);
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">
              {isTenantView ? "我的上架统计" : "上架统计"}
            </h1>
            <p className="mt-1 text-sm text-zinc-500">
              {isTenantView
                ? `当前账号可见的自动上架店铺共 ${total} 个，完成 ${summary.completed_count} / ${summary.daily_limit}，待处理 ${summary.remaining_count}。`
                : `共 ${total} 个自动上架店铺，完成 ${summary.completed_count} / ${summary.daily_limit}，待处理 ${summary.remaining_count}。`}
            </p>
          </div>
          <form
            className="flex flex-col gap-2 sm:flex-row sm:flex-wrap"
            onSubmit={handleSubmit}
          >
            <Label className="mb-3 block text-xs font-medium text-zinc-500">
              日期
              <Input
                className="mt-1 h-9"
                type="date"
                value={date}
                onChange={(event) => handleDateChange(event.target.value)}
              />
            </Label>
            <Button
              type="submit"
              className="w-full sm:mt-5 sm:w-auto"
              variant="secondary"
            >
              {loading ? (
                <RefreshCw className="size-4 animate-spin" />
              ) : (
                <Search className="size-4" />
              )}
              刷新
            </Button>
          </form>
        </div>
        {visibleError ? (
          <Alert className="mt-4" variant="destructive">
            <AlertDescription>{visibleError}</AlertDescription>
          </Alert>
        ) : null}
      </section>

      <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <StatTile label="完成数" value={String(summary.completed_count)} />
        <StatTile label="待处理" value={String(summary.remaining_count)} />
        <StatTile label="队列中" value={String(summary.queued_count)} />
        <StatTile label="挂起" value={String(summary.hold_count)} />
      </section>

      <section className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
        <div className="overflow-x-auto">
          <Table className="min-w-[56rem]">
            <TableHeader className="bg-zinc-50">
              <TableRow className="text-xs uppercase tracking-[0.2em] hover:bg-transparent">
                <TableHead>店铺</TableHead>
                <TableHead>店铺 ID</TableHead>
                <TableHead>平台</TableHead>
                <TableHead>额度</TableHead>
                <TableHead>任务</TableHead>
                <TableHead>进度</TableHead>
                <TableHead>状态</TableHead>
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
                    暂无统计数据
                  </TableCell>
                </TableRow>
              ) : (
                items.map((item) => (
                  <StoreStatisticsRow
                    key={item.id}
                    item={item}
                    isTenantView={isTenantView}
                  />
                ))
              )}
            </TableBody>
          </Table>
        </div>
        <StatisticsPagination
          page={currentPage}
          pageSize={pageSize}
          total={total}
          onPageChange={handlePageChange}
          onPageSizeChange={handlePageSizeChange}
        />
      </section>
    </div>
  );
}

function StoreStatisticsRow({
  item,
  isTenantView,
}: {
  item: ListingStoreStatistics;
  isTenantView: boolean;
}) {
  return (
    <TableRow className="align-top">
      <TableCell>
        <div className="font-medium text-zinc-950">{item.name}</div>
        <div className="font-mono text-xs text-zinc-500">
          {item.storeId || `#${item.id}`}
        </div>
        {isTenantView ? null : (
          <div className="text-xs text-zinc-500">租户 {item.tenantId}</div>
        )}
      </TableCell>
      <TableCell className="font-mono text-zinc-700">{item.id}</TableCell>
      <TableCell className="text-zinc-700">{item.platform || "-"}</TableCell>
      <TableCell className="text-zinc-700">
        <div>{item.dailyLimit}</div>
        <div className="text-xs text-zinc-500">
          {LIMIT_TYPE_TEXT[item.dailyLimitType ?? ""] ??
            item.dailyLimitType ??
            "-"}
        </div>
      </TableCell>
      <TableCell className="text-zinc-700">
        <div>
          {item.completedCount} / {item.dailyLimit}
        </div>
        <div className="text-xs text-zinc-500">
          待处理 {item.remainingCount}，队列中 {item.queuedCount}，挂起{" "}
          {item.holdCount}
        </div>
      </TableCell>
      <TableCell>
        <div className="flex min-w-44 items-center gap-2">
          <div className="h-2 flex-1 rounded-full bg-zinc-100">
            <div
              className="h-2 rounded-full bg-zinc-950"
              style={{
                width: `${Math.min(100, Math.max(0, item.progressPercentage))}%`,
              }}
            />
          </div>
          <span className="w-12 text-right text-xs font-medium text-zinc-700">
            {formatPercent(item.progressPercentage)}
          </span>
        </div>
        <div className="mt-1 text-xs text-zinc-500">
          剩余额度 {item.remainingQuota}
        </div>
      </TableCell>
      <TableCell>
        <Badge className="rounded-full px-2 py-1 text-xs" variant="neutral">
          {STATUS_TEXT[item.status] ?? `状态 ${item.status}`}
        </Badge>
      </TableCell>
    </TableRow>
  );
}

function StatisticsPagination({
  page,
  pageSize,
  total,
  onPageChange,
  onPageSizeChange,
}: {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
}) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const start = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const end = Math.min(total, page * pageSize);

  return (
    <div className="flex flex-col gap-3 border-t border-zinc-200 px-4 py-3 text-sm text-zinc-600 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
      <div>
        第 {page} / {totalPages} 页 · 显示 {start}-{end} / {total} 条
      </div>
      <div className="grid gap-2 sm:flex sm:flex-wrap sm:items-center">
        <Label className="flex items-center gap-2">
          <span>每页</span>
          <Select
            aria-label="每页"
            className="h-9 w-full rounded-xl px-2 text-sm sm:w-auto"
            value={pageSize}
            onChange={(event) => onPageSizeChange(Number(event.target.value))}
          >
            {PAGE_SIZE_OPTIONS.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </Select>
        </Label>
        <Button
          className="w-full sm:w-auto"
          variant="outline"
          size="sm"
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
          type="button"
        >
          上一页
        </Button>
        <Button
          className="w-full sm:w-auto"
          variant="outline"
          size="sm"
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
          type="button"
        >
          下一页
        </Button>
      </div>
    </div>
  );
}

function emptySummary(): ListingStoreStatisticsSummary {
  return {
    completed_count: 0,
    daily_limit: 0,
    remaining_count: 0,
    queued_count: 0,
    hold_count: 0,
  };
}

function formatPercent(value: number) {
  if (Number.isInteger(value)) {
    return `${value}%`;
  }
  return `${value.toFixed(2)}%`;
}

function StatTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
      <div className="mb-2 flex items-center gap-2 text-xs font-medium text-zinc-500">
        <BarChart3 className="size-4" />
        {label}
      </div>
      <div className="text-2xl font-semibold text-zinc-950">{value}</div>
    </div>
  );
}
