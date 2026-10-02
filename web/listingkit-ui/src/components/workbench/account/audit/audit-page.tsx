"use client";

import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { AccountReadError } from "@/lib/api/account";
import { auditContentInvalidReason, getAccountAudit, getAccountAuditSummary } from "@/lib/api/account-audit";
import { ConsoleState } from "../../console/console-page";
import styles from "./audit.module.css";

// The Account Shell owns heading, breadcrumbs and navigation.
export function auditRowKey(item: { eventType: string; actor: string; relation: { type: string; reference: string; version: string } }) {
  return `${item.eventType}:${item.relation.type}:${item.relation.reference}:${item.relation.version}:${item.actor}`;
}

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
  const [scopeError, setScopeError] = useState<string | null>(null);
  return <div className={styles.page}>
    <Card className={styles.coverage}>
      <h2>企业操作审计</h2>
      <p>只读取当前企业已提交的账户资料、成员、额度、模型实际用量、图片 AI 点数扣款与源账号事件。图片生成成功即扣点，后续未采用或图片处理失败不退还；未确认生成的预留不列为扣款。模型用量记录不提供操作人，按操作人筛选时不显示。汇总仅计成功操作，不计用量观察；资源覆盖续费期数与数据分配/回收、AI 月限和图片扣点，尚未包含独立店铺续费与支付记录。</p>
    </Card>
    <div className={styles.toolbar}><span>当前企业：{organizationId}</span><Button variant="outline" onClick={() => { setScopeError(null); setSequence(value => value + 1); }}>刷新记录</Button></div>
    {scopeError ? <AuditError code={scopeError} /> : <>
      <AuditSummary key={`summary:${sequence}`} scope={`${scope}:${sequence}`} expectedUserId={expectedUserId} organizationId={organizationId} onScopeRejected={setScopeError} />
      <AuditRequests key={sequence} scope={`${scope}:${sequence}`} expectedUserId={expectedUserId} organizationId={organizationId} onScopeRejected={setScopeError} />
    </>}
  </div>;
}
function useAuditScopeRejection(error: unknown, onScopeRejected: (code: string) => void) {
  useEffect(() => {
    if (error instanceof AccountReadError && ["AUTHENTICATION_REQUIRED", "IDENTITY_CONTEXT_CHANGED", "PERMISSION_DENIED", "ORGANIZATION_ACCESS_DENIED", "ORGANIZATION_ACCESS_REVOKED", "ORGANIZATION_SUSPENDED", "ORGANIZATION_CONTEXT_CHANGED", "ORGANIZATION_SELECTION_REQUIRED"].includes(error.code)) onScopeRejected(error.code);
  }, [error, onScopeRejected]);
}
function AuditSummary({ scope, expectedUserId, organizationId, onScopeRejected }: { scope: string; expectedUserId: string; organizationId: string; onScopeRejected: (code: string) => void }) {
  const query = useQuery({ queryKey: ["account-audit-summary", scope], queryFn: ({ signal }) => getAccountAuditSummary({ expectedUserId, expectedOrganizationId: organizationId, signal }), retry: false, staleTime: 0, gcTime: 0, refetchOnWindowFocus: true, refetchOnReconnect: true });
  useAuditScopeRejection(query.error, onScopeRejected);
  const data = !query.isPending && !query.isFetching && !query.isError ? query.data : undefined;
  const code = query.error instanceof AccountReadError ? query.error.code : "DEPENDENCY_UNAVAILABLE";
  const missing = code === "SUMMARY_NOT_CONFIGURED" || code === "ACCOUNT_NOT_CONFIGURED";
  const denied = ["AUTHENTICATION_REQUIRED", "PERMISSION_DENIED", "ORGANIZATION_ACCESS_REVOKED", "ORGANIZATION_ACCESS_DENIED", "ORGANIZATION_SUSPENDED"].includes(code);
  const unavailable = query.isPending || query.isFetching ? "读取中" : missing ? "未配置" : "读取失败";
  const metrics = [
    ["operations", "近 30 天操作", "当前范围内已提交的成功操作"],
    ["members", "成员变更", "邀请、移除与角色调整"],
    ["permissions", "权限变更", "角色调整，已包含于成员变更"],
    ["resources", "资源与续费", "期数/数据分配回收、月限与图片扣点"],
  ] as const;
  return <section aria-label="审计汇总">
    <div className={styles.metrics}>{metrics.map(([key, title, description]) => <article key={key}><span>{title}</span><strong>{data ? BigInt(data.counts[key]).toLocaleString("zh-CN") : unavailable}</strong><small>{description}</small></article>)}</div>
    {data ? <p className={styles.summaryWindow}>统计区间：<time dateTime={data.window.from}>{new Date(data.window.from).toLocaleString("zh-CN", { timeZone: "Asia/Singapore", hour12: false })}</time> 至 <time dateTime={data.window.asOf}>{new Date(data.window.asOf).toLocaleString("zh-CN", { timeZone: "Asia/Singapore", hour12: false })}</time>（不含截止时刻，UTC+8）。四项均为近 30 天完整记录，不随列表筛选或页码变化。</p>
      : query.isError ? <p className={styles.summaryWindow} role="status">{denied ? "汇总读取权限已失效" : missing ? "汇总来源尚未配置完整" : code === "DEADLINE_EXCEEDED" ? "汇总读取超时" : "汇总暂不可用"}，本次未取得完整统计。</p> : null}
  </section>;
}
function AuditRequests({ scope, expectedUserId, organizationId, onScopeRejected }: { scope: string; expectedUserId: string; organizationId: string; onScopeRejected: (code: string) => void }) {
  const [cursors, setCursors] = useState<(string | undefined)[]>([undefined]);
  type Operation = "" | "register" | "enable" | "disable" | "allocate_member_resource" | "reclaim_member_resource" | "set_member_ai_point_limit" | "update" | "invite" | "role" | "remove";
  type Period = "7d" | "30d" | "all";
  const [content, setContent] = useState("");
  const [period, setPeriod] = useState<Period>("30d");
  const [memberId, setMemberId] = useState("");
  const [actor, setActor] = useState("");
  const [operation, setOperation] = useState<Operation>("");
  const [applied, setApplied] = useState<{ content: string; period: Period; memberId: string; actor: string; operation: Operation }>({ content: "", period: "30d", memberId: "", actor: "", operation: "" });
  const [filterError, setFilterError] = useState("");
  const cursor = cursors[cursors.length - 1];
  const query = useQuery({ queryKey: ["account-audit", scope, cursor, applied], queryFn: ({ signal }) => getAccountAudit({ expectedUserId, expectedOrganizationId: organizationId, cursor, actor: applied.actor || undefined, operation: applied.operation || undefined, content: applied.content || undefined, memberId: applied.memberId || undefined, period: applied.period, signal }), retry: false, staleTime: 0, gcTime: 0, refetchOnWindowFocus: true, refetchOnReconnect: true });
  useAuditScopeRejection(query.error, onScopeRejected);
  if (query.isPending || query.isFetching) return <ConsoleState kind="loading" title="正在读取操作记录">正在确认访问权限。</ConsoleState>;
  if (query.isError) return <AuditError code={query.error instanceof AccountReadError ? query.error.code : "DEPENDENCY_UNAVAILABLE"} />;
  const data = query.data;
  const operationNames = { register: "登记源账号", enable: "启用源账号", disable: "停用源账号", allocate_member_resource: "分配成员资源", reclaim_member_resource: "回收成员资源", set_member_ai_point_limit: "设置成员 AI 月度上限", update: "更新账户资料", invite: "邀请成员", role: "更新成员角色", remove: "移除成员" };
  return <>
    <form className={styles.filters} onSubmit={event => { event.preventDefault(); const normalized = content.trim(); const invalid = auditContentInvalidReason(content); if (invalid === "control" || invalid === "too_long") { setFilterError(invalid === "control" ? "搜索词不能包含控制字符。" : "搜索词不能超过 80 字节。" ); return; } setFilterError(""); setApplied({ content: normalized, period, memberId, actor, operation }); setCursors([undefined]); }}>
      <label className={styles.search}>搜索操作内容 / 对象 <input value={content} onChange={event => setContent(event.target.value)} maxLength={80} placeholder="操作名称或对象 ID" /></label>
      <label>时间范围 <select value={period} onChange={event => setPeriod(event.target.value as Period)}><option value="7d">近7天</option><option value="30d">近30天</option><option value="all">全部时间</option></select></label>
      <label>操作类型 <select value={operation} onChange={event => setOperation(event.target.value as Operation)}><option value="">全部</option><option value="update">更新账户资料</option><option value="invite">邀请成员</option><option value="role">更新成员角色</option><option value="remove">移除成员</option><option value="register">登记源账号</option><option value="enable">启用源账号</option><option value="disable">停用源账号</option><option value="allocate_member_resource">分配成员资源</option><option value="reclaim_member_resource">回收成员资源</option><option value="set_member_ai_point_limit">设置成员 AI 月度上限</option></select></label>
      <label>成员筛选（成员 ID） <input value={memberId} onChange={event => setMemberId(event.target.value)} maxLength={128} pattern="[A-Za-z0-9][A-Za-z0-9._:-]{0,127}" placeholder="全部成员；可输入历史 ID" /></label>
      <label>操作人 <input value={actor} onChange={event => setActor(event.target.value)} maxLength={128} pattern="[A-Za-z0-9][A-Za-z0-9._:-]{0,127}" placeholder="按操作人筛选" /></label>
      <Button type="submit" variant="outline">应用筛选</Button>
    </form>
    <p className={styles.filterHint}>成员筛选按受影响或消耗资源的成员 ID；已移除成员可输入历史记录中的原 ID。操作人单独筛选。搜索覆盖已显示的操作名称、对象及资源信息，不包含姓名和联系方式。</p>
    {filterError ? <p role="alert" className={styles.filterError}>{filterError}</p> : null}
    <Card className={styles.panel}>
      <div className={styles.scroll} tabIndex={0} role="region" aria-label="操作记录表格，可横向滚动">
        <table className={styles.table} aria-label="操作记录"><thead><tr><th scope="col">时间</th><th scope="col">操作人</th><th scope="col">操作内容</th><th scope="col">模块</th><th scope="col">结果</th></tr></thead>
          <tbody>{data.items.length === 0 ? <tr><td colSpan={5} className={styles.emptyCell}>当前范围内没有已提交的操作。</td></tr> : data.items.map(item => <tr key={auditRowKey(item)}>
            <td><time dateTime={item.time}>{new Date(item.time).toLocaleString("zh-CN", { timeZone: "Asia/Singapore", hour12: false })}<small>UTC+8</small></time></td>
            <td><span>{item.actor || "未记录操作人"}</span></td>
            <td>{item.eventType === "account_member_resource.changed" || item.eventType === "account_member_ai_point_limit.changed" ? <><strong>{item.eventType === "account_member_ai_point_limit.changed" ? `设置成员 AI 月度上限：${item.resource.quantity} 点/月` : `${item.operation === "allocate_member_resource" ? "分配" : "回收"}${item.resource.type === "store_renewal_period" ? "续费期数" : "数据额度"}：${item.resource.quantity} ${item.resource.type === "store_renewal_period" ? "期" : "条"}`}</strong><small>成员 {item.objectReference} · 操作 {item.relation.reference}</small><small>操作版本 {item.relation.version}</small></> : item.eventType === "account_ai_points.committed" ? <><strong>图片 AI 点数已扣：{item.points.quantity}</strong><small>成员 {item.points.memberId} · 生成 {item.objectReference}</small><small>价格版本 {item.points.priceVersion} · 账本事件 {item.relation.reference}</small></> : item.eventType === "ai_invocation.usage_observed" ? <><strong>模型实际用量：{item.usage.quantity} Token</strong><small>成员 {item.usage.memberId} · 调用 {item.objectReference}</small><small>调用记录 {item.relation.reference}</small></> : <><strong>{operationNames[item.operation as keyof typeof operationNames] ?? item.operation}</strong><small>{item.objectType === "source_account" ? "源账号" : item.objectType === "account_business_profile" ? "账户资料" : item.objectType === "organization_member" ? "成员" : "额度"} {item.objectReference}</small><small>操作版本 {item.relation.version}</small></>}</td>
            <td><span className={styles.module}>{item.objectType === "source_account" ? "源账号" : item.objectType === "account_business_profile" ? "账户" : item.objectType === "organization_member" ? "成员与权限" : "资源与额度"}</span></td>
            <td><span className={styles.success}>{item.result === "observed" ? "已记录用量" : "已完成"}</span></td>
          </tr>)}</tbody>
        </table>
      </div>
    </Card>
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
