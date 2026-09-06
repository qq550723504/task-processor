"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import type { CompletedWorkItem } from "@/lib/api/completed-work";
import { fetchCompletedWork } from "@/lib/api/completed-work-client";
import { ConsoleState } from "../console/console-page";
import { CompletedWorkResults, WorkResultDetail } from "./completed-work-results";
import { TaskCenterLayout } from "./task-center-layout";

const PAGE_SIZE = 20;
const HISTORY_LIMIT = 50;
type Position = { cursors: (string | undefined)[]; page: number; sequence: number };

export function CompletedWorkPageContent({ available, completed = false }: { available: boolean; completed?: boolean }) {
  const context = useWorkbenchContext();
  if (!available) return <TaskCenterLayout completed={completed} />;
  if (context.isSwitching) return <TaskCenterLayout completed={completed}><ConsoleState kind="loading" title="正在切换企业">已清空工作记录和选择。</ConsoleState></TaskCenterLayout>;
  if (context.error || context.blockingError || !context.user || !context.effectiveOrganization || context.selectionRequired || context.isLoading) {
    return <TaskCenterLayout completed={completed}><ConsoleState kind="unavailable" title="企业或登录上下文不可用">已停止读取工作记录并清空选择。</ConsoleState></TaskCenterLayout>;
  }
  const scope = JSON.stringify([context.user.id, context.effectiveOrganization.id, context.roles]);
  return <ScopedWork key={scope} scope={scope} organizationId={context.effectiveOrganization.id} completed={completed} />;
}

function ScopedWork({ scope, organizationId, completed }: { scope: string; organizationId: string; completed: boolean }) {
  const [position, setPosition] = useState<Position>({ cursors: [undefined], page: 1, sequence: 0 });
  const refresh = () => setPosition((old) => ({ cursors: [undefined], page: 1, sequence: old.sequence + 1 }));
  const next = (cursor: string) => setPosition((old) => ({ cursors: [...old.cursors, cursor].slice(-HISTORY_LIMIT), page: old.page + 1, sequence: old.sequence + 1 }));
  const previous = () => setPosition((old) => old.cursors.length <= 1 ? old : ({ cursors: old.cursors.slice(0, -1), page: old.page - 1, sequence: old.sequence + 1 }));
  return <TaskCenterLayout completed={completed} onRefresh={refresh}>
    <WorkRequest key={position.sequence} scope={scope} organizationId={organizationId} position={position} refresh={refresh} next={next} previous={previous} />
  </TaskCenterLayout>;
}

function WorkRequest({ scope, organizationId, position, refresh, next, previous }: { scope: string; organizationId: string; position: Position; refresh: () => void; next: (cursor: string) => void; previous: () => void }) {
  const cursor = position.cursors.at(-1);
  const response = useQuery({
    queryKey: ["workbench", organizationId, "completed-work", scope, PAGE_SIZE, cursor, position.sequence],
    queryFn: ({ signal }) => fetchCompletedWork({ organizationId, limit: PAGE_SIZE, cursor, signal }),
    gcTime: 0, staleTime: 0, retry: false,
    refetchOnWindowFocus: false, refetchOnReconnect: false, refetchInterval: false,
  });
  const pageLabel = useRef<HTMLParagraphElement>(null);
  const ready = response.isSuccess && !response.isFetching;
  useEffect(() => { if (ready && position.sequence > 0) pageLabel.current?.focus(); }, [ready, position.sequence]);
  if (response.isPending || response.isFetching) return <ConsoleState kind="loading" title="正在读取工作记录">正在重新校验当前范围，旧结果与选择已隐藏。</ConsoleState>;
  if (response.isError) return <WorkError error={response.error} retry={refresh} />;
  if (!response.data) return <WorkError error={{ code: "INVALID_UPSTREAM_RESPONSE" }} retry={refresh} />;
  const data = response.data;
  return <CompletedWorkResults entries={data.items.map((item) => ({
    key: item.source_record_id, heading: item.title, caption: item.product_key,
    content: <span>{item.platform} · <time dateTime={item.created_at}>{item.created_at.replace("T", " ").replace(/Z$/, " UTC")}</time></span>,
    detail: <WorkDetail item={item} />,
  }))} pagination={<div className="mb-4 flex flex-wrap items-center gap-3">
    <p ref={pageLabel} tabIndex={-1} className="rounded text-xs focus-visible:outline-2 focus-visible:outline-ring" role="status">第 {position.page} 页</p>
    <Button size="sm" variant="outline" disabled={position.cursors.length <= 1} onClick={previous}>上一页</Button>
    <Button size="sm" variant="outline" disabled={!data.next_cursor} onClick={() => { if (data.next_cursor) next(data.next_cursor); }}>下一页</Button>
    <p className="w-full text-xs text-muted-foreground">每页最多 {PAGE_SIZE} 条，最新创建在前；刷新回到第一页。</p>
    {position.page > 1 && position.cursors.length === 1 ? <p className="text-xs text-muted-foreground">更早的分页位置已释放，可刷新回到第一页。</p> : null}
  </div>} />;
}

function WorkDetail({ item }: { item: CompletedWorkItem }) {
  return <WorkResultDetail title={item.title} summary={item.summary}>
    <dl><dt>来源商品</dt><dd>{item.product_key}</dd><dt>快照版本</dt><dd>{item.snapshot_version}</dd>
      <dt>平台</dt><dd>{item.platform}</dd><dt>国家 / 语言</dt><dd>{item.country} / {item.language}</dd>
      <dt>创建时间</dt><dd><time dateTime={item.created_at}>{item.created_at.replace("T", " ").replace(/Z$/, " UTC")}</time></dd>
      <dt>结果引用</dt><dd>{item.source_record_id}</dd></dl>
    <Button asChild variant="outline"><Link href={item.result.href} prefetch={false}>查看诊断</Link></Button>
  </WorkResultDetail>;
}

function WorkError({ error, retry }: { error: unknown; retry: () => void }) {
  const context = useWorkbenchContext();
  const mounted = useRef(false);
  const [recovering, setRecovering] = useState(false);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const code = error && typeof error === "object" && "code" in error && typeof error.code === "string" ? error.code : "REQUEST_FAILED";
  const messages: Record<string, string> = {
    PERMISSION_DENIED: "没有读取工作记录的权限", permission_denied: "没有读取工作记录的权限",
    AUTHENTICATION_REQUIRED: "登录状态已失效", ORGANIZATION_CONTEXT_CHANGED: "企业上下文已变化",
    ORGANIZATION_ACCESS_REVOKED: "当前企业访问已撤销", ORGANIZATION_ACCESS_DENIED: "当前企业访问被拒绝",
    ORGANIZATION_SUSPENDED: "当前企业已暂停访问", DEPENDENCY_UNAVAILABLE: "工作记录服务暂不可用", unavailable: "工作记录服务暂不可用",
    DEADLINE_EXCEEDED: "读取工作记录超时", deadline_exceeded: "读取工作记录超时",
    INVALID_UPSTREAM_RESPONSE: "工作记录响应不合法", invalid_request: "分页请求不合法", INVALID_REQUEST: "分页请求不合法",
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
  return <ConsoleState kind="error" title={messages[code] ?? "工作记录读取失败，请稍后重试"}>
    <p>本次未取得可用记录，旧结果与选择已隐藏。可刷新重新读取首页。</p>
    {contextError ? <Button variant="outline" className="mt-4" disabled={recovering} onClick={recover}>{recovering ? "正在恢复企业上下文…" : "重新加载企业上下文"}</Button> : null}
  </ConsoleState>;
}
