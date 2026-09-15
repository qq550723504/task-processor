"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { createSourceAccount, disableSourceAccount, enableSourceAccount, getSourceAccount, listSourceAccounts, SourceAccountAPIError, type CreateSourceAccountRequest, type ChangeSourceAccountStatusRequest } from "@/lib/api/source-accounts";
import { sourceAccountCreateRequestSchema } from "@/lib/contracts/source-account";
import { ConsoleState } from "../console/console-page";
import { AccountShell } from "../account/account-shell";
import styles from "./source-accounts.module.css";

type Intent = { action: "register"; request: CreateSourceAccountRequest } | { action: "enable" | "disable"; request: ChangeSourceAccountStatusRequest };
const queryOptions = { gcTime: 0, staleTime: 0, retry: false, refetchOnWindowFocus: true, refetchOnReconnect: true } as const;
const accessFailures = new Set(["AUTHENTICATION_REQUIRED", "PERMISSION_DENIED", "ORGANIZATION_ACCESS_REVOKED", "ORGANIZATION_ACCESS_DENIED", "ORGANIZATION_SUSPENDED", "ORGANIZATION_CONTEXT_CHANGED", "IDENTITY_CONTEXT_CHANGED"]);

export function SourceAccountsPage() {
  const context = useWorkbenchContext();
  // One in-memory submitted intent survives a context switch. It is never
  // rendered or replayed in another actor/organization and is not a ledger.
  const [submittedIntent, setSubmittedIntent] = useState<Intent | null>(null);
  const org = context.effectiveOrganization;
  const canManage = org?.capabilities?.["workbench.source_account.manage"] === true;
  const scope = JSON.stringify([context.user?.id, org?.id, context.roles, canManage]);
  const sameIntentScope = submittedIntent?.request.expectedOrganizationId === org?.id && submittedIntent?.request.expectedActorSubject === context.user?.id;
  return <AccountShell pathname="/workbench/account/organization/resources/source-accounts" title="源账号" description="可选企业资源：登记和启停仅管理本系统资源，匿名公开商品采集无需先登记或连接源账号。">
    <Button asChild variant="outline"><Link href="/workbench/account/organization/resources" prefetch={false}>返回资源与额度</Link></Button>
    {context.isLoading || context.isSwitching ? <ConsoleState kind="loading" title="正在确认当前企业">旧账号和待操作已清除。</ConsoleState>
      : context.error || context.blockingError || !context.user || !org || context.selectionRequired ? <ConsoleState kind="error" title="企业或登录上下文不可用">请确认当前企业与登录状态。<Button variant="outline" onClick={() => void context.retry()}>重新确认上下文</Button></ConsoleState>
      : <ScopedAccounts key={scope} scope={scope} organizationId={org.id} organizationName={org.name} actor={context.user.id} canManage={canManage} initialIntent={sameIntentScope ? submittedIntent : null} foreignIntent={!!submittedIntent && !sameIntentScope} onIntentChange={setSubmittedIntent} />}
  </AccountShell>;
}

function ScopedAccounts({ scope, organizationId, organizationName, actor, canManage, initialIntent, foreignIntent, onIntentChange }: { scope: string; organizationId: string; organizationName: string; actor: string; canManage: boolean; initialIntent: Intent | null; foreignIntent: boolean; onIntentChange: (intent: Intent | null) => void }) {
  const [sequence, setSequence] = useState(0);
  const [selected, setSelected] = useState<string | null>(null);
  const [displayName, setDisplayName] = useState("");
  const [pending, setPending] = useState(false);
  const [unknown, setUnknown] = useState<Intent | null>(initialIntent);
  const [error, setError] = useState<string | null>(initialIntent ? "OUTCOME_UNKNOWN" : null);
  const [success, setSuccess] = useState(false);
  const [accessError, setAccessError] = useState<string | null>(null);
  const active = useRef(true);
  const busy = useRef(false);
  const abort = useRef<AbortController | null>(null);
  useEffect(() => { active.current = true; return () => { active.current = false; abort.current?.abort(); }; }, []);

  async function execute(intent: Intent) {
    if (busy.current || !active.current || foreignIntent || !canManage) return;
    onIntentChange(intent);
    busy.current = true; setPending(true); setError(null); setSuccess(false);
    const controller = new AbortController(); abort.current = controller;
    const timeout = setTimeout(() => controller.abort(), 15000);
    try {
      const result = intent.action === "register" ? await createSourceAccount({ ...intent.request, signal: controller.signal }) : await (intent.action === "enable" ? enableSourceAccount : disableSourceAccount)({ ...intent.request, signal: controller.signal });
      if (!active.current) return;
      setUnknown(null); onIntentChange(null); setSuccess(true); setSelected(result.account.id); setSequence(v => v + 1);
      if (intent.action === "register") setDisplayName("");
    } catch (failure) {
      if (!active.current) return;
      // Rejection of a later verification request says nothing about whether
      // the original request committed. Only its successful receipt resolves it.
      if (unknown || !(failure instanceof SourceAccountAPIError) || failure.outcome === "unknown") {
        setUnknown(intent); setError("OUTCOME_UNKNOWN");
        if (failure instanceof SourceAccountAPIError && accessFailures.has(failure.code)) setAccessError(failure.code);
      } else {
        setUnknown(null); onIntentChange(null); setError(failure.code);
        if (accessFailures.has(failure.code)) { setSelected(null); setDisplayName(""); setAccessError(failure.code); }
      }
    } finally {
      clearTimeout(timeout); busy.current = false;
      if (active.current) setPending(false);
    }
  }

  function register() {
    if (!canManage || unknown || foreignIntent || busy.current) return;
    const parsed = sourceAccountCreateRequestSchema.safeParse({ displayName, platform: "1688" });
    if (!parsed.success) { setError("INVALID_REQUEST"); return; }
    void execute({ action: "register", request: { ...parsed.data, expectedOrganizationId: organizationId, expectedActorSubject: actor, idempotencyKey: crypto.randomUUID() } });
  }

  if (accessError) return <div className={styles.stack}><SourceError code={accessError} /><Button variant="outline" onClick={() => { setAccessError(null); setError(null); setSelected(null); setSequence(v => v + 1); }}>刷新列表</Button></div>;
  return <div className={styles.stack}>
    <p>当前有效企业：{organizationName || "未提供名称"}（{organizationId}）</p>
    <p className={styles.note}>管理状态、平台连接状态和采集能力分别判断。本页不提供平台登录或连接操作；待连接不表示登记错误。</p>
    {foreignIntent ? <p role="status">另一个企业或身份下有尚未确认的操作。请恢复原上下文后处理，再发起新操作。</p> : null}
    {error ? <SourceError code={error} /> : null}
    {unknown ? <Card className={styles.panel}><p>原操作结果尚未确定。重试会再次提交原请求；若原操作尚未提交，本次可能完成原操作。保留相同操作标识与内容，不发起新操作；刷新列表和详情仅为只读。</p>{canManage ? <Button variant="outline" disabled={pending} onClick={() => void execute(unknown)}>使用原请求重试</Button> : null}<p>离开或刷新页面不会把未知结果判为失败；返回后先读取服务端现状，再决定后续操作。</p></Card> : null}
    {pending ? <p role="status">正在提交，请勿重复操作。</p> : null}
    {success ? <p role="status">操作已确认，正在重新读取当前状态；请以下方读取结果为准。</p> : null}
    {canManage ? <Card className={styles.panel}><h2>登记源账号</h2><form onSubmit={event => { event.preventDefault(); register(); }} className={styles.form}>
      <label htmlFor="source-account-name">显示名称</label><Input id="source-account-name" value={displayName} onChange={event => setDisplayName(event.target.value)} disabled={pending || !!unknown || foreignIntent} required maxLength={120} aria-describedby="source-account-name-help" />
      <p id="source-account-name-help">名称最多 120 UTF-8 字节，不含首尾空格或控制字符。平台：1688。</p>
      <Button type="submit" disabled={pending || !!unknown || foreignIntent}>登记源账号</Button>
    </form></Card> : <p>当前角色只读；管理权限由服务端确认。</p>}
    <div className={styles.actions}><Button variant="outline" disabled={pending} onClick={() => { setSequence(v => v + 1); setSuccess(false); }}>刷新列表</Button></div>
    <AccountList key={`list:${sequence}`} scope={scope} organizationId={organizationId} sequence={sequence} onSelect={setSelected} onAccessError={setAccessError} />
    {selected ? <AccountDetail key={`${selected}:${sequence}`} scope={scope} organizationId={organizationId} id={selected} sequence={sequence} onAccessError={setAccessError} canManage={canManage && !unknown && !pending && !foreignIntent} onChange={(action, ifMatch) => { if (!unknown && !busy.current && canManage) void execute({ action, request: { sourceAccountId: selected, ifMatch, expectedOrganizationId: organizationId, expectedActorSubject: actor, idempotencyKey: crypto.randomUUID() } }); }} /> : null}
  </div>;
}

function AccountList({ scope, organizationId, sequence, onSelect, onAccessError }: { scope: string; organizationId: string; sequence: number; onSelect: (id: string) => void; onAccessError: (code: string) => void }) {
  const [cursors, setCursors] = useState<(string | undefined)[]>([undefined]);
  const cursor = cursors.at(-1);
  const response = useQuery({ ...queryOptions, queryKey: ["workbench", organizationId, "source-account-list", scope, sequence, cursor], queryFn: ({ signal }) => listSourceAccounts({ expectedOrganizationId: organizationId, limit: 20, ...(cursor ? { cursor } : {}), signal }) });
  useAccessFailure(response.error, onAccessError);
  if (response.isPending || response.isFetching) return <ConsoleState kind="loading" title="正在读取源账号">正在确认当前企业权限。</ConsoleState>;
  if (response.isError) return <SourceError code={response.error instanceof SourceAccountAPIError ? response.error.code : "INVALID_UPSTREAM_RESPONSE"} />;
  return <Card className={styles.panel}><h2>源账号列表</h2>
    {response.data.items.length ? <ul className={styles.list}>{response.data.items.map(row => <li key={row.id}>
      <div><h3>{row.displayName}</h3><p>平台：{row.platform}</p><p>管理状态：<span>{row.managementStatus === "enabled" ? "已启用" : "已禁用"}</span></p><p>平台连接：<span>{row.connectionStatus === "pending_connection" ? "待连接（未验证）" : "未提供"}</span></p></div>
      <Button variant="outline" aria-label={`查看 ${row.displayName}`} onClick={() => onSelect(row.id)}>查看详情</Button>
    </li>)}</ul> : <p>尚未登记源账号</p>}
    <div className={styles.actions}><Button variant="outline" disabled={cursors.length === 1} onClick={() => setCursors(v => v.slice(0, -1))}>上一页</Button><Button variant="outline" disabled={!response.data.nextCursor} onClick={() => setCursors(v => [...v, response.data.nextCursor!])}>下一页</Button></div>
  </Card>;
}

function AccountDetail({ scope, organizationId, id, sequence, canManage, onChange, onAccessError }: { scope: string; organizationId: string; id: string; sequence: number; canManage: boolean; onChange: (action: "enable" | "disable", etag: string) => void; onAccessError: (code: string) => void }) {
  const response = useQuery({ ...queryOptions, queryKey: ["workbench", organizationId, "source-account-detail", scope, id, sequence], queryFn: ({ signal }) => getSourceAccount({ expectedOrganizationId: organizationId, sourceAccountId: id, signal }) });
  useAccessFailure(response.error, onAccessError);
  if (response.isPending || response.isFetching) return <ConsoleState kind="loading" title="正在读取账号详情">正在确认当前权限和版本。</ConsoleState>;
  if (response.isError) return <SourceError code={response.error instanceof SourceAccountAPIError ? response.error.code : "INVALID_UPSTREAM_RESPONSE"} />;
  const row = response.data.account;
  return <Card className={styles.panel} role="region" aria-label="源账号详情"><h2>源账号详情</h2>
    <dl className={styles.facts}>{[["显示名称", row.displayName], ["平台", row.platform], ["管理状态", row.managementStatus === "enabled" ? "已启用" : "已禁用"], ["平台连接", row.connectionStatus === "pending_connection" ? "待连接（未验证）" : "未提供"], ["采集能力", "未提供独立验证结果"], ["版本", row.version], ["创建时间", row.createdAt], ["更新时间", row.updatedAt]].map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>
    {canManage ? <Button variant="outline" onClick={() => onChange(row.managementStatus === "enabled" ? "disable" : "enable", response.data.etag)}>{row.managementStatus === "enabled" ? "禁用源账号" : "启用源账号"}</Button> : null}
  </Card>;
}

function useAccessFailure(error: Error | null, onAccessError: (code: string) => void) {
  useEffect(() => { if (error instanceof SourceAccountAPIError && accessFailures.has(error.code)) onAccessError(error.code); }, [error, onAccessError]);
}

function SourceError({ code }: { code: string }) {
  const labels: Record<string, string> = { OUTCOME_UNKNOWN: "结果待核实", INVALID_REQUEST: "请检查显示名称", VERSION_CONFLICT: "账号版本已变化，请刷新后确认", IDEMPOTENCY_CONFLICT: "操作内容冲突", INVALID_TRANSITION: "当前状态不允许此操作", RESOURCE_LIMIT_REACHED: "资源数量已达限制", PERMISSION_DENIED: "无操作权限", AUTHENTICATION_REQUIRED: "登录已失效", ORGANIZATION_ACCESS_REVOKED: "企业访问已撤销", ORGANIZATION_ACCESS_DENIED: "企业访问被拒绝", ORGANIZATION_CONTEXT_CHANGED: "当前企业已变化", IDENTITY_CONTEXT_CHANGED: "当前身份已变化", DEPENDENCY_UNAVAILABLE: "源账号服务暂不可用", SOURCE_ACCOUNT_NOT_FOUND: "当前企业中未找到该账号", DEADLINE_EXCEEDED: "请求超时" };
  return <ConsoleState kind="error" title={labels[code] ?? "源账号请求失败"}>未据此推断账号状态。请确认当前身份和企业，再刷新读取；已提交但未确认的操作请使用原请求重试。</ConsoleState>;
}
