"use client";

import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { AccountReadError } from "@/lib/api/account";
import { getAccountAudit } from "@/lib/api/account-audit";
import { ConsoleState } from "../../console/console-page";
import styles from "./audit.module.css";

// The Account Shell owns heading, breadcrumbs and navigation.
export function AuditPage({ expectedUserId }: { expectedUserId: string }) {
  const context = useWorkbenchContext();
  const [leaving, setLeaving] = useState(false);
  useEffect(() => {
    const onClick = (event: MouseEvent) => {
      const link = event.target instanceof Element ? event.target.closest("a") : null;
      if (link && new URL(link.href, window.location.href).pathname === "/api/zitadel-auth/logout") setLeaving(true);
    };
    document.addEventListener("click", onClick, true);
    return () => document.removeEventListener("click", onClick, true);
  }, []);
  if (leaving || context.user && context.user.id !== expectedUserId) return <AuditError code="AUTHENTICATION_REQUIRED" />;
  if (context.isLoading || context.isSwitching) return <ConsoleState kind="loading" title="正在确认当前企业">旧企业记录已清除。</ConsoleState>;
  if (context.error || context.blockingError || !context.user || !context.effectiveOrganization || context.selectionRequired) return <AuditError code={context.blockingError?.code ?? context.error?.code ?? "ORGANIZATION_SELECTION_REQUIRED"} />;
  const scope = JSON.stringify([expectedUserId, context.user.id, context.effectiveOrganization.id, context.roles]);
  return <ScopedAudit key={scope} scope={scope} expectedUserId={expectedUserId} organizationId={context.effectiveOrganization.id} />;
}
function ScopedAudit({ scope, expectedUserId, organizationId }: { scope: string; expectedUserId: string; organizationId: string }) {
  const [sequence, setSequence] = useState(0);
  return <div className={styles.page}>
    <Card className={styles.coverage}>
      <h2>源账号已提交操作</h2>
      <p>当前展示源账号的登记、启用和停用记录。成员、权限、续费及失败尝试尚未纳入。</p>
      <p>时间为业务操作时间；记录只供追溯，管理动作请前往对应资源页面。</p>
    </Card>
    <div className={styles.toolbar}><span>当前企业：{organizationId}</span><Button variant="outline" onClick={() => setSequence(value => value + 1)}>刷新记录</Button></div>
    <AuditRequests key={sequence} scope={`${scope}:${sequence}`} expectedUserId={expectedUserId} organizationId={organizationId} />
  </div>;
}
function AuditRequests({ scope, expectedUserId, organizationId }: { scope: string; expectedUserId: string; organizationId: string }) {
  const [cursors, setCursors] = useState<(string | undefined)[]>([undefined]);
  const cursor = cursors[cursors.length - 1];
  const query = useQuery({ queryKey: ["account-audit", scope, cursor], queryFn: ({ signal }) => getAccountAudit({ expectedUserId, expectedOrganizationId: organizationId, cursor, signal }), retry: false, staleTime: 0, gcTime: 0, refetchOnWindowFocus: true, refetchOnReconnect: true });
  if (query.isPending || query.isFetching) return <ConsoleState kind="loading" title="正在读取操作记录">正在确认访问权限。</ConsoleState>;
  if (query.isError) return <AuditError code={query.error instanceof AccountReadError ? query.error.code : "DEPENDENCY_UNAVAILABLE"} />;
  const data = query.data;
  const operationNames = { register: "登记源账号", enable: "启用源账号", disable: "停用源账号" };
  return <>
    {data.items.length === 0 ? <ConsoleState kind="empty" title="暂无操作记录">当前范围内没有已提交的源账号操作。</ConsoleState> : <Card className={styles.panel}>
      <div className={styles.scroll} tabIndex={0} role="region" aria-label="操作记录表格，可横向滚动">
        <table className={styles.table} aria-label="操作记录"><thead><tr><th scope="col">时间</th><th scope="col">操作人</th><th scope="col">操作内容</th><th scope="col">模块</th><th scope="col">结果</th></tr></thead>
          <tbody>{data.items.map(item => <tr key={`${item.objectReference}:${item.relation.version}`}>
            <td><time dateTime={item.time}>{new Date(item.time).toLocaleString("zh-CN", { timeZone: "Asia/Singapore", hour12: false })}<small>UTC+8</small></time></td>
            <td><span>{item.actor}</span></td>
            <td><strong>{operationNames[item.operation]}</strong><small>源账号 {item.objectReference}</small><small>操作版本 {item.relation.version}</small></td>
            <td><span className={styles.module}>资源与额度</span></td>
            <td><span className={styles.success}>已完成</span></td>
          </tr>)}</tbody>
        </table>
      </div>
    </Card>}
    <nav className={styles.pagination} aria-label="操作记录分页">
      <span>第 {cursors.length} 页 · 本页 {data.items.length} 条</span>
      <Button variant="outline" disabled={cursors.length === 1} onClick={() => setCursors(value => value.slice(0, -1))}>上一页</Button>
      <Button variant="outline" disabled={!data.nextCursor} onClick={() => { if (data.nextCursor) setCursors(value => [...value, data.nextCursor!]); }}>下一页</Button>
    </nav>
  </>;
}
function AuditError({ code }: { code: string }) {
  const titles: Record<string, string> = { AUTHENTICATION_REQUIRED: "登录已失效", IDENTITY_CONTEXT_CHANGED: "登录身份已变化", PERMISSION_DENIED: "无查看操作记录权限", ORGANIZATION_ACCESS_DENIED: "企业访问被拒绝", ORGANIZATION_ACCESS_REVOKED: "企业访问已撤销", ORGANIZATION_SUSPENDED: "企业访问已暂停", ORGANIZATION_SELECTION_REQUIRED: "请选择当前企业", ORGANIZATION_CONTEXT_CHANGED: "当前企业已变化", DEADLINE_EXCEEDED: "操作记录读取超时" };
  return <ConsoleState kind="error" title={titles[code] ?? "操作记录暂不可用"}>本次未取得记录。请确认登录和企业状态后刷新。</ConsoleState>;
}
