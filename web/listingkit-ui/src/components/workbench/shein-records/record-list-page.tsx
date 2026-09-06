"use client";

import { useQuery } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight, RefreshCw } from "lucide-react";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import type { SheinRecordListItem } from "@/lib/api/shein-records";
import { fetchSheinRecords } from "@/lib/api/shein-records-client";

const PAGE_SIZE = 20;
const HISTORY_LIMIT = 50;
type Position = { cursors: (string | undefined)[]; page: number; sequence: number };

export function SheinRecordListPage({ available }: { available: boolean }) {
  const context = useWorkbenchContext();
  const scope = JSON.stringify([context.user?.id, context.effectiveOrganization?.id, context.roles]);
  return (
    <section className="mx-auto w-full max-w-6xl px-4 py-6 sm:px-6 sm:py-8">
      <Link href="/workbench" prefetch={false} className="rounded text-sm underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-ring">返回工作台</Link>
      <h1 className="mt-5 text-2xl font-semibold tracking-tight">SHEIN 本地资料</h1>
      <p className="mt-2 text-sm leading-6 text-muted-foreground">查看本地保存的资料及离线诊断。这里不是平台商品列表，资料不代表已发布或归属某个店铺。</p>
      {!available ? <Card className="mt-6 p-6"><h2 className="font-semibold">暂未启用</h2><p className="mt-2 text-sm text-muted-foreground">当前环境尚未装配本地资料读取能力。</p></Card>
        : context.isSwitching ? <ContextState message="正在切换企业，已清空资料列表…" />
        : context.error || context.blockingError || !context.user || !context.effectiveOrganization || context.selectionRequired || context.isLoading
          ? <ContextState message="企业或登录上下文不可用，已停止读取资料。" />
          : <ScopedRecords key={scope} organizationId={context.effectiveOrganization.id} organizationName={context.effectiveOrganization.name} scope={scope} />}
    </section>
  );
}

function ContextState({ message }: { message: string }) {
  return <p className="mt-6 text-sm" role="status">{message}</p>;
}

function ScopedRecords({ organizationId, organizationName, scope }: { organizationId: string; organizationName: string; scope: string }) {
  const [position, setPosition] = useState<Position>({ cursors: [undefined], page: 1, sequence: 0 });
  const refresh = () => setPosition((previous) => ({ cursors: [undefined], page: 1, sequence: previous.sequence + 1 }));
  const next = (cursor: string) => setPosition((previous) => ({ cursors: [...previous.cursors, cursor].slice(-HISTORY_LIMIT), page: previous.page + 1, sequence: previous.sequence + 1 }));
  const previous = () => setPosition((current) => current.cursors.length <= 1 ? current : ({ cursors: current.cursors.slice(0, -1), page: current.page - 1, sequence: current.sequence + 1 }));
  return (
    <>
      <div className="my-5 flex flex-wrap items-center justify-between gap-3">
        <p className="break-words text-sm font-medium [overflow-wrap:anywhere]">{organizationName}</p>
        <Button variant="outline" onClick={refresh}><RefreshCw aria-hidden="true" />刷新列表</Button>
      </div>
      <ListRequest key={position.sequence} organizationId={organizationId} scope={scope} position={position} refresh={refresh} next={next} previous={previous} />
    </>
  );
}

function ListRequest({ organizationId, scope, position, refresh, next, previous }: { organizationId: string; scope: string; position: Position; refresh: () => void; next: (cursor: string) => void; previous: () => void }) {
  const cursor = position.cursors.at(-1);
  const result = useQuery({
    queryKey: ["workbench", organizationId, "shein-records", scope, PAGE_SIZE, cursor, position.sequence],
    queryFn: ({ signal }) => fetchSheinRecords({ organizationId, limit: PAGE_SIZE, cursor, signal }),
    gcTime: 0, staleTime: 0, retry: false,
    refetchOnWindowFocus: false, refetchOnReconnect: false, refetchInterval: false,
  });
  const pageLabel = useRef<HTMLParagraphElement>(null);
  const ready = result.isSuccess && !result.isFetching;
  useEffect(() => { if (ready && position.sequence > 0) pageLabel.current?.focus(); }, [ready, position.sequence]);
  if (result.isPending || result.isFetching) return <Card className="p-6" role="status" aria-live="polite">正在读取本地资料…</Card>;
  if (result.isError) return <ListError error={result.error} retry={refresh} />;
  if (!result.data) return <ListError error={{ code: "INVALID_UPSTREAM_RESPONSE" }} retry={refresh} />;
  const records = result.data;
  return (
    <>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <p ref={pageLabel} tabIndex={-1} className="rounded text-sm focus-visible:outline-2 focus-visible:outline-ring" role="status">第 {position.page} 页</p>
        <div className="flex gap-2">
          <Button size="sm" variant="outline" disabled={position.cursors.length <= 1} onClick={previous}><ChevronLeft aria-hidden="true" />上一页</Button>
          <Button size="sm" variant="outline" disabled={!records.next_cursor} onClick={() => { if (records.next_cursor) next(records.next_cursor); }}>下一页<ChevronRight aria-hidden="true" /></Button>
        </div>
        <p className="w-full text-xs text-muted-foreground">最新创建的资料在前，每页最多 {PAGE_SIZE} 条。刷新会回到第一页。</p>
        {position.page > 1 && position.cursors.length === 1 ? <p className="w-full text-xs text-muted-foreground">更早的分页位置已释放，可刷新回到首页。</p> : null}
      </div>
      {records.items.length === 0 ? <Card className="p-6 sm:p-8"><h2 className="font-semibold">当前范围内暂无本地资料</h2><p className="mt-2 text-sm leading-6 text-muted-foreground">当前企业与账号可读取的本地资料为空；这不表示 SHEIN 平台没有商品。</p></Card>
        : <Card className="overflow-hidden"><ul aria-label="本地资料" className="divide-y divide-border">{records.items.map((record) => <RecordRow key={record.record_id} record={record} />)}</ul></Card>}
    </>
  );
}

function RecordRow({ record }: { record: SheinRecordListItem }) {
  return (
    <li className="grid min-w-0 gap-4 p-4 [overflow-wrap:anywhere] sm:p-5 md:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_auto] md:items-center">
      <div className="min-w-0">
        <p className="text-xs text-muted-foreground">来源商品标识</p>
        <h2 className="mt-1 break-words text-sm font-semibold">{record.product_key}</h2>
        <dl className="mt-3 flex flex-wrap gap-x-2 gap-y-1 text-xs"><dt className="text-muted-foreground">快照版本</dt><dd className="font-mono">{record.snapshot_version}</dd></dl>
      </div>
      <dl className="grid min-w-0 gap-2 text-xs">
        <div className="flex flex-wrap gap-2"><dt className="text-muted-foreground">国家 / 语言</dt><dd>{record.country} / {record.language}</dd></div>
        <div><dt className="text-muted-foreground">创建时间</dt><dd className="mt-1"><time dateTime={record.created_at}>{record.created_at.replace("T", " ").replace(/Z$/, " UTC")}</time></dd></div>
      </dl>
      <Button asChild variant="outline" size="sm" className="justify-self-start"><Link href={`/workbench/shein-records/${record.record_id}/diagnostic`} prefetch={false}>查看诊断</Link></Button>
    </li>
  );
}

function ListError({ error, retry }: { error: unknown; retry: () => void }) {
  const context = useWorkbenchContext();
  const mounted = useRef(false);
  const [recovering, setRecovering] = useState(false);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const code = error && typeof error === "object" && "code" in error && typeof error.code === "string" ? error.code : "REQUEST_FAILED";
  const messages: Record<string, string> = {
    PERMISSION_DENIED: "没有读取本地资料的权限", permission_denied: "没有读取本地资料的权限",
    AUTHENTICATION_REQUIRED: "登录状态已失效",
    ORGANIZATION_CONTEXT_CHANGED: "企业上下文已变化", ORGANIZATION_ACCESS_REVOKED: "当前企业访问已撤销",
    ORGANIZATION_ACCESS_DENIED: "当前企业访问被拒绝", ORGANIZATION_SUSPENDED: "当前企业已暂停访问",
    DEPENDENCY_UNAVAILABLE: "资料服务暂不可用", unavailable: "资料服务暂不可用",
    DEADLINE_EXCEEDED: "读取资料超时", deadline_exceeded: "读取资料超时",
    INVALID_UPSTREAM_RESPONSE: "资料列表响应不合法", invalid_request: "分页请求不合法", INVALID_REQUEST: "分页请求不合法",
  };
  const contextError = code.startsWith("ORGANIZATION_") || code === "AUTHENTICATION_REQUIRED";
  async function recover() {
    setRecovering(true);
    const recovered = await context.retry();
    if (!mounted.current) return;
    setRecovering(false);
    const organization = recovered?.organizations.find((item) => item.id === recovered.effectiveOrganizationId);
    if (recovered && !recovered.selectionRequired && recovered.user.id === context.user?.id && organization?.id === context.effectiveOrganization?.id && JSON.stringify(organization?.roles) === JSON.stringify(context.roles)) retry();
  }
  return <Card className="p-6"><div role="alert"><h2 className="font-semibold">{messages[code] ?? "资料读取失败，请稍后重试"}</h2><p className="mt-2 text-sm text-muted-foreground">本次未取得可用列表，旧结果已隐藏。刷新列表可重新读取首页。</p></div>{contextError ? <Button variant="outline" className="mt-4" disabled={recovering} onClick={recover}>{recovering ? "正在恢复企业上下文…" : "重新加载企业上下文"}</Button> : null}</Card>;
}
