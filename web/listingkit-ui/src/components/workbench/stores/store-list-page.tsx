"use client";

import { ChevronLeft, ChevronRight, Plus, RefreshCw } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { StoreTable } from "@/components/workbench/stores/store-table";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { WorkbenchAPIError, type WorkbenchStoreListFilters } from "@/lib/api/workbench-stores";
import { useWorkbenchStores } from "@/lib/query/use-workbench-stores";
import { canCreateWorkbenchStore } from "@/lib/workbench/permissions";

const DEFAULT_PAGE = 1;
const DEFAULT_PAGE_SIZE = 20;
export function StoreListPage() {
  const pathname = usePathname() || "/workbench/stores";
  const router = useRouter();
  const searchParams = useSearchParams();
  const context = useWorkbenchContext();
  const filters = parseStoreFilters(searchParams);
  const stores = useWorkbenchStores(filters);
  const data = stores.data;
  const hasFilters = Boolean(filters.platform || filters.status);
  const deletedNotice = searchParams.getAll("notice").length === 1 && searchParams.get("notice") === "store-deleted";
  const canCreateByRole = canCreateWorkbenchStore(context.roles);
  const canCreate = Boolean(data && canCreateByRole);

  const updateFilters = (next: WorkbenchStoreListFilters) => {
    router.push(`${pathname}?${buildStoreSearch(next)}`);
  };
  const resetFilters = () =>
    updateFilters({ page: DEFAULT_PAGE, pageSize: filters.pageSize });
  const retryStores = () => {
    const code = stores.error instanceof WorkbenchAPIError ? stores.error.code : (stores.error as { code?: string } | null)?.code;
    if (
      code === "ORGANIZATION_CONTEXT_CHANGED" ||
      code === "ORGANIZATION_ACCESS_REVOKED" ||
      code === "ORGANIZATION_ACCESS_DENIED"
    ) {
      context.retry();
      return;
    }
    void stores.refetch();
  };

  return (
    <section className="mx-auto w-full max-w-7xl px-4 py-8 sm:px-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="text-sm text-muted-foreground">店铺中心</p>
          <h1 className="text-2xl font-semibold tracking-tight">我的店铺</h1>
          {context.effectiveOrganization ? (
            <p className="mt-1 text-sm text-muted-foreground">{context.effectiveOrganization.name}</p>
          ) : null}
        </div>
        {canCreate ? (
          <Button asChild>
            <Link href="/workbench/stores/new"><Plus aria-hidden="true" />新建店铺</Link>
          </Button>
        ) : null}
      </div>

      <div className="mt-6 flex flex-wrap items-end gap-3 rounded-xl border bg-card p-4">
        <label className="grid gap-1 text-sm font-medium">
          平台
          <Select aria-label="平台" value={filters.platform ?? ""} onChange={(event) => updateFilters({ ...filters, page: 1, platform: event.target.value === "shein" ? "shein" : undefined })}>
            <option value="">全部平台</option><option value="shein">SHEIN</option>
          </Select>
        </label>
        <label className="grid gap-1 text-sm font-medium">
          店铺状态
          <Select aria-label="店铺状态" value={filters.status ?? ""} onChange={(event) => updateFilters({ ...filters, page: 1, status: isStoreStatus(event.target.value) ? event.target.value : undefined })}>
            <option value="">全部状态</option><option value="provisioning">开通中</option><option value="active">已启用</option><option value="disabled">已停用</option><option value="deleting">删除中</option>
          </Select>
        </label>
        <label className="grid gap-1 text-sm font-medium">
          每页数量
          <Select aria-label="每页数量" value={String(filters.pageSize)} onChange={(event) => updateFilters({ ...filters, page: 1, pageSize: parsePageSize(event.target.value) })}>
            {[10, 20, 50, 100].map((size) => <option key={size} value={size}>{size}</option>)}
          </Select>
        </label>
        {hasFilters && data?.items.length !== 0 ? <Button onClick={resetFilters} variant="ghost">清除筛选</Button> : null}
      </div>

      {deletedNotice ? <p className="mt-4 rounded-md border p-3 text-sm" role="status">店铺已删除。界面不提供恢复；如需运营恢复，请由管理员通过数据库软删除恢复流程处理。</p> : null}

      {stores.isPending ? <LoadingState /> : stores.isError ? (
        <ErrorState error={stores.error} retry={retryStores} />
      ) : data ? (
        <div className="mt-6 space-y-4">
          <QuotaSummary quota={data.quota} />
          {data.items.length === 0 ? (
            <EmptyState filtered={hasFilters} clearFilters={resetFilters} />
          ) : (
            <StoreTable onDeleted={() => router.push("/workbench/stores?notice=store-deleted")} onRefreshStore={async (storeId) => {
              const result = await stores.refetch();
              if (!result.isSuccess || result.isError) return null;
              return result.data?.items.find((store) => store.id === storeId) ?? null;
            }} stores={data.items} />
          )}
          <Pagination filters={filters} onChange={updateFilters} page={data.pagination.page} pageSize={data.pagination.pageSize} total={data.pagination.total} />
        </div>
      ) : null}
    </section>
  );
}

function LoadingState() {
  return <div className="mt-6 rounded-xl border bg-card p-6" role="status">正在加载店铺...</div>;
}

function ErrorState({ error, retry }: { error: unknown; retry: () => void }) {
  const code = error instanceof WorkbenchAPIError ? error.code : (error as { code?: string })?.code;
  const message = code === "PERMISSION_DENIED" ? "没有查看当前企业店铺的权限" : code === "ORGANIZATION_ACCESS_REVOKED" || code === "ORGANIZATION_ACCESS_DENIED" || code === "ORGANIZATION_SUSPENDED" ? "当前企业访问已不可用，请联系管理员" : "店铺服务暂时不可用，请稍后重试";
  return <section className="mt-6 rounded-xl border bg-card p-6" role="alert"><h2 className="font-semibold">{message}</h2><Button className="mt-4" onClick={retry} variant="outline"><RefreshCw aria-hidden="true" />重试</Button></section>;
}

function EmptyState({ filtered, clearFilters }: { filtered: boolean; clearFilters: () => void }) {
  return <section className="rounded-xl border bg-card p-8 text-center"><h2 className="font-semibold">{filtered ? "没有符合筛选条件的店铺" : "还没有店铺"}</h2><p className="mt-2 text-sm text-muted-foreground">{filtered ? "调整或清除筛选条件后再试。" : "新建店铺后会显示在这里。"}</p>{filtered ? <Button className="mt-4" onClick={clearFilters} variant="outline">清除筛选</Button> : null}</section>;
}

function QuotaSummary({ quota }: { quota: { used: number; limit: number | null; allowed: boolean; reason: string } }) {
  const hint = quota.reason === "subscription_required" ? "当前企业需要管理员配置有效订阅后才能新建店铺。" : quota.reason === "store_limit_reached" || (!quota.allowed && quota.limit !== null && quota.used >= quota.limit) ? "店铺额度已用尽，请联系管理员或升级套餐。" : null;
  return <div className="flex flex-wrap items-center justify-between gap-2 text-sm"><p>已使用 {quota.used} / {quota.limit ?? "—"}</p><p className="text-muted-foreground">已停用店铺仍占用店铺额度。</p>{hint ? <p className="text-muted-foreground">{hint}</p> : null}</div>;
}

function Pagination({ filters, onChange, page, pageSize, total }: { filters: WorkbenchStoreListFilters; onChange: (filters: WorkbenchStoreListFilters) => void; page: number; pageSize: number; total: number }) {
  return <div className="flex items-center justify-between gap-3"><p className="text-sm text-muted-foreground">第 {page} 页，共 {total} 家店铺</p><div className="flex gap-2"><Button aria-label="上一页" disabled={page <= 1} onClick={() => onChange({ ...filters, page: page - 1 })} size="sm" variant="outline"><ChevronLeft aria-hidden="true" />上一页</Button><Button aria-label="下一页" disabled={page * pageSize >= total} onClick={() => onChange({ ...filters, page: page + 1 })} size="sm" variant="outline">下一页<ChevronRight aria-hidden="true" /></Button></div></div>;
}

function parseStoreFilters(searchParams: URLSearchParams): WorkbenchStoreListFilters {
  const page = parseCanonicalDecimal(searchParams.getAll("page"), DEFAULT_PAGE, Number.MAX_SAFE_INTEGER);
  const pageSize = parseCanonicalDecimal(searchParams.getAll("pageSize"), DEFAULT_PAGE_SIZE, 100);
  const platform = searchParams.getAll("platform").length === 1 && searchParams.get("platform") === "shein" ? "shein" : undefined;
  const statusValue = searchParams.getAll("status").length === 1 ? searchParams.get("status") : null;
  return { page, pageSize, ...(platform ? { platform } : {}), ...(isStoreStatus(statusValue) ? { status: statusValue } : {}) };
}

function parseCanonicalDecimal(values: string[], fallback: number, maximum: number) {
  if (values.length !== 1 || !/^[1-9]\d*$/.test(values[0] ?? "")) return fallback;
  const parsed = Number(values[0]);
  return Number.isSafeInteger(parsed) && parsed <= maximum ? parsed : fallback;
}

function parsePageSize(value: string) {
  return parseCanonicalDecimal([value], DEFAULT_PAGE_SIZE, 100);
}

function isStoreStatus(value: string | null): value is NonNullable<WorkbenchStoreListFilters["status"]> {
  return value === "provisioning" || value === "active" || value === "disabled" || value === "deleting";
}

function buildStoreSearch(filters: WorkbenchStoreListFilters) {
  const params = new URLSearchParams();
  params.set("page", String(filters.page));
  params.set("pageSize", String(filters.pageSize));
  if (filters.platform) params.set("platform", filters.platform);
  if (filters.status) params.set("status", filters.status);
  return params.toString();
}
