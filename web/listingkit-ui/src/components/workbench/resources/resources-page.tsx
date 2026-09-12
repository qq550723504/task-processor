"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { getCommercialOverview } from "@/lib/api/commercial";
import { ConsolePage, ConsoleState } from "../console/console-page";
import { EntitlementsOverview } from "../commercial/commercial-views";
import styles from "./resources.module.css";

export function ResourcesPage() {
  const context = useWorkbenchContext();
  const org = context.effectiveOrganization;
  const scope = JSON.stringify([context.user?.id, org?.id, context.roles]);
  const valid = context.user && org && !context.selectionRequired && !context.error && !context.blockingError;
  return <ConsolePage title="资源与额度" className={styles.page} description="查看当前企业资源、已授予权益与可选资源管理。">
    {context.isLoading || context.isSwitching ? <ConsoleState kind="loading" title="正在确认当前企业">旧资源信息已清除。</ConsoleState>
      : !valid ? <ConsoleState kind="error" title="企业或登录上下文不可用">请确认登录状态并重新选择企业。<Button variant="outline" onClick={() => void context.retry()}>重新确认上下文</Button></ConsoleState>
      : <ScopedResources key={scope} scope={scope} organizationId={org.id} organizationName={org.name} />}
  </ConsolePage>;
}

function ScopedResources({ scope, organizationId, organizationName }: { scope: string; organizationId: string; organizationName: string }) {
  const [sequence, setSequence] = useState(0);
  return <div className={styles.stack}>
    <div className={styles.heading}><p>当前有效企业：{organizationName || "未提供名称"}（{organizationId}）</p><Button variant="outline" onClick={() => setSequence(v => v + 1)}>刷新资源</Button></div>
    <div className={styles.grid}>
      <Card role="region" aria-label="源账号资源" className={styles.panel}><h2>源账号</h2><p>可选企业资源，可登记和管理来源账号；匿名公开商品采集无需先登记或连接源账号。</p><Button asChild variant="outline"><Link href="/workbench/account/organization/resources/source-accounts" prefetch={false}>管理源账号</Link></Button></Card>
      <Card role="region" aria-label="店铺资源" className={styles.panel}><h2>店铺资源</h2><p>实际店铺数量未提供，店铺服务及平台连接状态未接入。</p><p>店铺数量限制是已授予权益，不代表已绑定或服务中的店铺数量。</p></Card>
    </div>
    <ResourceRequest key={sequence} scope={scope} organizationId={organizationId} sequence={sequence} />
  </div>;
}

function ResourceRequest({ scope, organizationId, sequence }: { scope: string; organizationId: string; sequence: number }) {
  const response = useQuery({ queryKey: ["workbench", organizationId, "account-resources", scope, sequence], queryFn: ({ signal }) => getCommercialOverview(organizationId, signal), gcTime: 0, staleTime: 0, retry: false, refetchOnWindowFocus: true, refetchOnReconnect: true });
  if (response.isPending || response.isFetching) return <ConsoleState kind="loading" title="正在读取权益与用量">正在确认当前企业权限，旧数据已隐藏。</ConsoleState>;
  if (response.isError || !response.data || response.data.organization_id !== organizationId) {
    const code = response.error && "code" in response.error ? String(response.error.code) : "INVALID_UPSTREAM_RESPONSE";
    const labels: Record<string, string> = { PERMISSION_DENIED: "无权益查看权限", AUTHENTICATION_REQUIRED: "登录已失效", ORGANIZATION_ACCESS_REVOKED: "企业访问已撤销", ORGANIZATION_ACCESS_DENIED: "企业访问被拒绝", DEPENDENCY_UNAVAILABLE: "权益服务暂不可用", DEADLINE_EXCEEDED: "读取超时" };
    return <ConsoleState kind="error" title={labels[code] ?? "权益读取失败"}>本次未取得数据，不能判断权益、用量或余额。请刷新资源重新读取。</ConsoleState>;
  }
  return <EntitlementsOverview data={response.data} />;
}
