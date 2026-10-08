"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { getCommercialOverview } from "@/lib/api/commercial";
import { listWorkbenchStores } from "@/lib/api/workbench-stores";
import { ConsoleState } from "../console/console-page";
import { EntitlementsOverview } from "../commercial/commercial-views";
import { EnterpriseResources } from "../commercial/enterprise-resources";
import { AccountShell } from "../account/account-shell";
import { MemberPointLimits } from "./member-point-limits";
import { MemberResources } from "./member-resources";
import styles from "./resources.module.css";

export function ResourcesPage() {
  const context = useWorkbenchContext();
  const org = context.effectiveOrganization;
  const scope = JSON.stringify([context.user?.id, org?.id, context.roles]);
  const readScope = JSON.stringify([context.user?.id, org?.id, context.roles, context.permissions]);
  const valid = context.user && org && !context.selectionRequired && !context.error && !context.blockingError;
  return <AccountShell pathname="/workbench/account/organization/resources" title="资源与额度" description="查看当前企业资源、已授予权益与可选资源管理。">
    {context.isLoading || context.isSwitching ? <ConsoleState kind="loading" title="正在确认当前企业">旧资源信息已清除。</ConsoleState>
      : !valid || !context.user ? <ConsoleState kind="error" title="企业或登录上下文不可用">请确认登录状态并重新选择企业。<Button variant="outline" onClick={() => void context.retry()}>重新确认上下文</Button></ConsoleState>
      : <ScopedResources key={scope} scope={readScope} userId={context.user.id} organizationId={org.id} organizationName={org.name} canManage={context.roles.some(role => ["listingkit_admin", "platform_admin", "admin"].includes(role))} />}
  </AccountShell>;
}

function ScopedResources({ scope, userId, organizationId, organizationName, canManage }: { scope: string; userId: string; organizationId: string; organizationName: string; canManage: boolean }) {
  const [sequence, setSequence] = useState(0);
  return <div className={styles.stack}>
    <div className={styles.heading}><p>当前有效企业：{organizationName || "未提供名称"}（{organizationId}）</p><Button variant="outline" onClick={() => setSequence(v => v + 1)}>刷新资源</Button></div>
    <section aria-label="企业资源权益摘要"><EnterpriseResources userId={userId} organizationId={organizationId} scope={scope} sequence={sequence} /></section>
    <MemberResources userId={userId} organizationId={organizationId} sequence={sequence} canManage={canManage}/>
    <MemberPointLimits userId={userId} organizationId={organizationId} sequence={sequence} canManage={canManage} />
    <div className={styles.grid}>
      <Card role="region" aria-label="源账号资源" className={styles.panel}><h2>源账号</h2><p>可选企业资源，可登记和管理来源账号；匿名公开商品采集无需先登记或连接源账号。</p><Button asChild variant="outline"><Link href="/workbench/account/organization/resources/source-accounts" prefetch={false}>管理源账号</Link></Button></Card>
      <StoreResources scope={scope} userId={userId} organizationId={organizationId} sequence={sequence} />
    </div>
    <ResourceRequest key={sequence} scope={scope} organizationId={organizationId} sequence={sequence} />
  </div>;
}

function StoreResources({ scope, userId, organizationId, sequence }: { scope: string; userId: string; organizationId: string; sequence: number }) {
  const stores = useQuery({ queryKey: ["workbench", userId, organizationId, "account-store-count", scope, sequence], queryFn: ({ signal }) => listWorkbenchStores({ page: 1, pageSize: 1 }, organizationId, signal), gcTime: 0, staleTime: 0, retry: false });
  return <Card role="region" aria-label="店铺资源" className={styles.panel}>
    <h2>店铺资源</h2>
    {stores.isPending || stores.isFetching ? <p role="status">正在读取店铺记录数…</p> :
      stores.isError || !stores.data ? <p role="alert">本次未取得店铺记录数，请确认当前企业后重试。</p> :
        <dl><dt>店铺记录数</dt><dd>{stores.data.pagination.total} 家</dd></dl>}
    <p>当前身份可查看的已授权店铺数量；服务期限与平台连接状态可在我的店铺查看。</p>
    <Button asChild variant="outline"><Link href="/workbench/stores" prefetch={false}>管理店铺</Link></Button>
  </Card>;
}

function ResourceRequest({ scope, organizationId, sequence }: { scope: string; organizationId: string; sequence: number }) {
  const response = useQuery({ queryKey: ["workbench", organizationId, "account-resources", scope, sequence], queryFn: ({ signal }) => getCommercialOverview(organizationId, signal), gcTime: 0, staleTime: 0, retry: false, refetchOnWindowFocus: true, refetchOnReconnect: true });
  if (response.isPending || response.isFetching) return <p role="status">正在确认当前企业权益权限；店铺服务与权益正在读取。</p>;
  if (response.isError || !response.data || response.data.organization_id !== organizationId) {
    const code = response.error && "code" in response.error ? String(response.error.code) : "INVALID_UPSTREAM_RESPONSE";
    const labels: Record<string, string> = { PERMISSION_DENIED: "无权益查看权限", AUTHENTICATION_REQUIRED: "登录已失效", ORGANIZATION_ACCESS_REVOKED: "企业访问已撤销", ORGANIZATION_ACCESS_DENIED: "企业访问被拒绝", DEPENDENCY_UNAVAILABLE: "权益服务暂不可用", DEADLINE_EXCEEDED: "读取超时" };
    return <p role="alert">{labels[code] ?? "权益读取失败"}：本次未取得店铺服务与权益；企业资源余额以上方读取结果为准。请刷新资源重新读取。</p>;
  }
  return <EntitlementsOverview data={response.data} />;
}
