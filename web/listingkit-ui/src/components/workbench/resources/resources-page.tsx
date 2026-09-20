"use client";

import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { getCommercialOverview } from "@/lib/api/commercial";
import { AccountAllocationError, getMemberTokenAllocations, setMemberTokenAllocation, type MemberTokenAllocationSnapshot } from "@/lib/api/account-allocation";
import { ConsoleState } from "../console/console-page";
import { EntitlementsOverview } from "../commercial/commercial-views";
import { AccountShell } from "../account/account-shell";
import styles from "./resources.module.css";

export function ResourcesPage() {
  const context = useWorkbenchContext();
  const org = context.effectiveOrganization;
  const scope = JSON.stringify([context.user?.id, org?.id, context.roles]);
  const valid = context.user && org && !context.selectionRequired && !context.error && !context.blockingError;
  return <AccountShell pathname="/workbench/account/organization/resources" title="资源与额度" description="查看当前企业资源、已授予权益与可选资源管理。">
    {context.isLoading || context.isSwitching ? <ConsoleState kind="loading" title="正在确认当前企业">旧资源信息已清除。</ConsoleState>
      : !valid ? <ConsoleState kind="error" title="企业或登录上下文不可用">请确认登录状态并重新选择企业。<Button variant="outline" onClick={() => void context.retry()}>重新确认上下文</Button></ConsoleState>
      : <ScopedResources key={scope} scope={scope} organizationId={org.id} organizationName={org.name} canManage={context.roles.some(role => ["listingkit_admin", "platform_admin", "admin"].includes(role))} />}
  </AccountShell>;
}

function ScopedResources({ scope, organizationId, organizationName, canManage }: { scope: string; organizationId: string; organizationName: string; canManage: boolean }) {
  const [sequence, setSequence] = useState(0);
  return <div className={styles.stack}>
    <div className={styles.heading}><p>当前有效企业：{organizationName || "未提供名称"}（{organizationId}）</p><Button variant="outline" onClick={() => setSequence(v => v + 1)}>刷新资源</Button></div>
    <div className={styles.grid}>
      <Card role="region" aria-label="源账号资源" className={styles.panel}><h2>源账号</h2><p>可选企业资源，可登记和管理来源账号；匿名公开商品采集无需先登记或连接源账号。</p><Button asChild variant="outline"><Link href="/workbench/account/organization/resources/source-accounts" prefetch={false}>管理源账号</Link></Button></Card>
      <Card role="region" aria-label="店铺资源" className={styles.panel}><h2>店铺资源</h2><p>实际店铺数量未提供，店铺服务及平台连接状态未接入。</p><p>店铺数量限制是已授予权益，不代表已绑定或服务中的店铺数量。</p></Card>
    </div>
    <ResourceRequest key={sequence} scope={scope} organizationId={organizationId} sequence={sequence} canManage={canManage} />
  </div>;
}

function ResourceRequest({ scope, organizationId, sequence, canManage }: { scope: string; organizationId: string; sequence: number; canManage: boolean }) {
  const response = useQuery({ queryKey: ["workbench", organizationId, "account-resources", scope, sequence], queryFn: ({ signal }) => getCommercialOverview(organizationId, signal), gcTime: 0, staleTime: 0, retry: false, refetchOnWindowFocus: true, refetchOnReconnect: true });
  if (response.isPending || response.isFetching) return <ConsoleState kind="loading" title="正在读取权益与用量">正在确认当前企业权限，旧数据已隐藏。</ConsoleState>;
  if (response.isError || !response.data || response.data.organization_id !== organizationId) {
    const code = response.error && "code" in response.error ? String(response.error.code) : "INVALID_UPSTREAM_RESPONSE";
    const labels: Record<string, string> = { PERMISSION_DENIED: "无权益查看权限", AUTHENTICATION_REQUIRED: "登录已失效", ORGANIZATION_ACCESS_REVOKED: "企业访问已撤销", ORGANIZATION_ACCESS_DENIED: "企业访问被拒绝", DEPENDENCY_UNAVAILABLE: "权益服务暂不可用", DEADLINE_EXCEEDED: "读取超时" };
    return <ConsoleState kind="error" title={labels[code] ?? "权益读取失败"}>本次未取得数据，不能判断权益、用量或余额。请刷新资源重新读取。</ConsoleState>;
  }
  return <><EntitlementsOverview data={response.data} /><MemberTokenAllocationRequest organizationId={organizationId} sequence={sequence} canManage={canManage} /></>;
}

function MemberTokenAllocationRequest({ organizationId, sequence, canManage }: { organizationId: string; sequence: number; canManage: boolean }) {
  const response = useQuery({ queryKey: ["workbench", organizationId, "member-token-allocations", sequence], queryFn: ({ signal }) => getMemberTokenAllocations(organizationId, signal), gcTime: 0, staleTime: 0, retry: false, refetchOnWindowFocus: true, refetchOnReconnect: true });
  if (response.isPending || response.isFetching) return <ConsoleState kind="loading" title="正在读取成员 Token 分配">旧分配信息已隐藏。</ConsoleState>;
  if (response.isError || !response.data) {
    const code = response.error instanceof AccountAllocationError ? response.error.code : "DEPENDENCY_UNAVAILABLE";
    return <ConsoleState kind="error" title={code === "CONFLICT" ? "成员分配已发生变化" : "成员 Token 分配暂不可用"}>本次未取得企业额度与成员分配，不能显示或修改旧数据。</ConsoleState>;
  }
  return <MemberTokenAllocationTable data={response.data} organizationId={organizationId} canManage={canManage} />;
}

function MemberTokenAllocationTable({ data, organizationId, canManage }: { data: MemberTokenAllocationSnapshot; organizationId: string; canManage: boolean }) {
  const client = useQueryClient();
  const [targets, setTargets] = useState<Record<string, string>>(() => Object.fromEntries(data.members.map(member => [member.memberId, member.allocation.allocated])));
  const [saving, setSaving] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const mutation = useMutation({ mutationFn: async ({ memberId, target, version }: { memberId: string; target: string; version: string }) => setMemberTokenAllocation(organizationId, memberId, target, version), onMutate: ({ memberId }) => { setSaving(memberId); setError(null); }, onSuccess: () => { void client.invalidateQueries({ queryKey: ["workbench", organizationId, "member-token-allocations"] }); }, onError: error => setError(error instanceof AccountAllocationError && error.code === "CONFLICT" ? "版本已过期，请刷新后重试。" : "分配未保存，企业额度事实未改变。"), onSettled: () => setSaving(null) });
  return <Card role="region" aria-label="成员 AI Token 分配" className={styles.panel}>
    <div className={styles.heading}><div><h2>成员 AI Token 分配</h2><p>分配是成员使用上限，不等于实际消费；周期继承当前企业权益窗口。</p></div><Button variant="outline" onClick={() => void client.invalidateQueries({ queryKey: ["workbench", organizationId, "member-token-allocations"] })}>刷新分配</Button></div>
    <dl className={styles.quotaFacts}><div><dt>企业总额度</dt><dd>{data.enterprise.total}</dd></div><div><dt>已分配</dt><dd>{data.enterprise.allocated}</dd></div><div><dt>未分配</dt><dd>{data.enterprise.unallocated}</dd></div><div><dt>企业已消费</dt><dd>{data.enterprise.consumed}</dd></div></dl>
    {error ? <p role="alert">{error}</p> : null}
    <div className={styles.memberRows}>{data.members.map(member => <div className={styles.memberRow} key={member.memberId}><div><strong>{member.displayName || member.loginName || member.userId}</strong><small>{member.loginName || member.userId}</small></div><dl><div><dt>已分配</dt><dd>{member.allocation.allocated}</dd></div><div><dt>已消费</dt><dd>{member.allocation.consumed}</dd></div><div><dt>剩余</dt><dd>{member.allocation.remaining}</dd></div></dl>{canManage ? <div className={styles.allocationEdit}><label htmlFor={`allocation-${member.memberId}`}>目标额度</label><input id={`allocation-${member.memberId}`} inputMode="numeric" pattern="[0-9]*" value={targets[member.memberId] ?? "0"} onChange={event => setTargets(current => ({ ...current, [member.memberId]: event.target.value }))} /><Button disabled={saving !== null} onClick={() => void mutation.mutate({ memberId: member.memberId, target: targets[member.memberId] ?? "0", version: member.allocation.version })}>{saving === member.memberId ? "保存中…" : "保存目标"}</Button></div> : <span className={styles.readOnly}>只读</span>}</div>)}</div>
  </Card>;
}
