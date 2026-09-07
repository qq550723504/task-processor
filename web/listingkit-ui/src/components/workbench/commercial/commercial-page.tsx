"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useState, type ReactNode } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { getCommercialOverview } from "@/lib/api/commercial";
import { ConsolePage, ConsoleState } from "../console/console-page";
import { EntitlementsOverview, PlanOptions } from "./commercial-views";
import styles from "./commercial.module.css";

type PageKind = "options" | "entitlements";

export function CommercialPage({ page }: { page: PageKind }) {
  const context = useWorkbenchContext();
  const organization = context.effectiveOrganization;
  if (context.isSwitching || context.isLoading) return <PageFrame page={page} organization="正在确认企业">
    <ConsoleState kind="loading" title={context.isSwitching ? "正在切换企业" : "正在读取企业上下文"}>旧商业数据已清空，观察时间与周期尚未取得。</ConsoleState>
  </PageFrame>;
  if (context.error || context.blockingError || !context.user || !organization || context.selectionRequired) return <PageFrame page={page} organization="未确认">
    <ConsoleState kind="unavailable" title="企业或登录上下文不可用">已停止读取商业数据。观察时间、周期与数值尚未取得。</ConsoleState>
  </PageFrame>;
  const scope = JSON.stringify([context.user.id, organization.id, context.roles]);
  return <ScopedCommercial key={`${page}:${scope}`} page={page} scope={scope} organizationId={organization.id} organizationName={organization.name} />;
}

function PageFrame({ page, organization, actions, children }: { page: PageKind; organization: string; actions?: ReactNode; children: ReactNode }) {
  const title = page === "options" ? "套餐方案" : "我的权益";
  return <ConsolePage className={styles.page} title={title} breadcrumbs={[{ label: "套餐与权益" }, { label: title }]} description={<>
    <p>{page === "options" ? "查看已批准的方案说明；实际订阅和已授予权益分别展示。" : "查看当前企业的订阅、已授予权益及已记录用量。"}</p>
    <p>当前有效企业：{organization}</p>
  </>} actions={actions}>{children}</ConsolePage>;
}

function ScopedCommercial({ page, scope, organizationId, organizationName }: { page: PageKind; scope: string; organizationId: string; organizationName: string }) {
  const [sequence, setSequence] = useState(0);
  const refresh = () => setSequence(value => value + 1);
  return <PageFrame page={page} organization={`${organizationName || "未提供名称"}（${organizationId}）`} actions={<>
    <Button asChild variant="outline"><Link prefetch={false} href={page === "options" ? "/workbench/plans/entitlements" : "/workbench/plans/options"}>{page === "options" ? "查看我的权益" : "查看套餐方案"}</Link></Button>
    <Button variant="outline" onClick={refresh}>刷新数据</Button>
  </>}>
    <CommercialRequest key={sequence} page={page} scope={scope} organizationId={organizationId} sequence={sequence} />
  </PageFrame>;
}

function CommercialRequest({ page, scope, organizationId, sequence }: { page: PageKind; scope: string; organizationId: string; sequence: number }) {
  const response = useQuery({
    queryKey: ["workbench", organizationId, "commercial", page, scope, sequence],
    queryFn: ({ signal }) => getCommercialOverview(organizationId, signal),
    gcTime: 0, staleTime: 0, retry: false,
    refetchOnWindowFocus: false, refetchOnReconnect: false, refetchInterval: false,
  });
  if (response.isPending || response.isFetching) return <ConsoleState kind="loading" title="正在读取商业数据">正在校验当前企业权限，旧结果已隐藏。观察时间与周期尚未取得。</ConsoleState>;
  if (response.isError) return <ReadError error={response.error} />;
  if (!response.data || response.data.organization_id !== organizationId) return <ReadError error={{ code: "INVALID_UPSTREAM_RESPONSE" }} />;
  return page === "options" ? <PlanOptions data={response.data} /> : <EntitlementsOverview data={response.data} />;
}

function ReadError({ error }: { error: unknown }) {
  const code = error && typeof error === "object" && "code" in error && typeof error.code === "string" ? error.code : "UNKNOWN";
  const messages: Record<string, string> = {
    PERMISSION_DENIED: "无查看权限", AUTHENTICATION_REQUIRED: "登录已失效",
    ORGANIZATION_ACCESS_REVOKED: "企业访问已撤销", ORGANIZATION_ACCESS_DENIED: "企业访问被拒绝",
    ORGANIZATION_SUSPENDED: "企业已暂停访问", ORGANIZATION_CONTEXT_CHANGED: "企业上下文已变化",
    ORGANIZATION_SELECTION_REQUIRED: "请选择当前企业", INVALID_REQUEST: "商业数据请求无效",
    DEPENDENCY_UNAVAILABLE: "商业数据依赖暂不可用", DEADLINE_EXCEEDED: "商业数据读取超时",
    INVALID_UPSTREAM_RESPONSE: "商业数据响应无效",
  };
  return <ConsoleState kind="error" title={messages[code] ?? "商业数据读取失败"}>
    <p>本次未取得数据，不能判断订阅、权益或余额。观察时间与周期未取得。</p>
    <p>可重新选择企业或刷新数据，再由服务端确认访问权限。</p>
  </ConsoleState>;
}
