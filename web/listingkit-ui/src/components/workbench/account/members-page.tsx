"use client";

import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { changeMemberRole, getMember, getMemberOperation, getMemberOperations, getMembers, invitationInput, inviteMember, MemberError, MemberOperation, MemberOperations, MemberRole, MemberScope, removeMember, verifyMemberOperation } from "@/lib/api/members";
import { Pending, readPending, retainAdvancedReceipt, savePending, terminalReceipt } from "./member-pending";
import { ConsoleState } from "../console/console-page";
import { AccountShell } from "./account-shell";
import styles from "./members.module.css";

const roleNames: Record<MemberRole, string> = { listingkit_viewer: "只读成员", listingkit_operator: "操作成员", listingkit_admin: "企业管理员" };
const subscribe = (notify: () => void) => { window.addEventListener("membership-pending", notify); window.addEventListener("storage", notify); return () => { window.removeEventListener("membership-pending", notify); window.removeEventListener("storage", notify); }; };
const noPending = () => null;
const isAuthorityFailure = (failure: unknown): failure is MemberError => failure instanceof MemberError && (failure.status === 401 || failure.status === 403 || ["IDENTITY_CONTEXT_CHANGED", "ORGANIZATION_CONTEXT_CHANGED", "ORGANIZATION_SELECTION_REQUIRED"].includes(failure.code));

export function MembersPage({ expectedUserId }: { expectedUserId: string }) {
  const context = useWorkbenchContext(); const [leaving, setLeaving] = useState(false);
  useEffect(() => { const click = (event: MouseEvent) => { const link = event.target instanceof Element ? event.target.closest("a") : null; if (link && new URL(link.href, location.href).pathname === "/api/zitadel-auth/logout") setLeaving(true); }; document.addEventListener("click", click, true); return () => document.removeEventListener("click", click, true); }, []);
  const org = context.effectiveOrganization;
  const scope = JSON.stringify([expectedUserId, context.user?.id, org?.id, context.roles, context.isLoading, context.error?.code, context.blockingError?.code]);
  let content;
  if (leaving || context.user?.id !== expectedUserId) content = <MemberFailure code="AUTHENTICATION_REQUIRED" />;
  else if (context.isSwitching || context.isLoading) content = <ConsoleState kind="loading" title="正在确认当前企业">旧成员资料已清除。</ConsoleState>;
  else if (!org || context.selectionRequired || context.error || context.blockingError) content = <MemberFailure code={context.blockingError?.code ?? context.error?.code ?? "ORGANIZATION_SELECTION_REQUIRED"} />;
  else content = <ScopedMembers key={scope} scope={{ expectedUserId, expectedOrganizationId: org.id }} />;
  return <AccountShell pathname="/workbench/account/organization/members" title="成员与权限" description="邀请成员、分配角色并查看当前企业的成员访问状态">{content}</AccountShell>;
}

function ScopedMembers({ scope }: { scope: MemberScope }) {
  const [offset, setOffset] = useState(0); const [selected, setSelected] = useState<string | null>(null); const [inviting, setInviting] = useState(false);
  const [busy, setBusy] = useState(false); const busyRef = useRef(false); const [error, setError] = useState("");
  const [receipts, setReceipts] = useState<Record<string, MemberOperation>>({});
  const remember = (items: MemberOperation[]) => setReceipts(previous => {
    const next = {...previous};
    for (const item of items) next[item.id] = retainAdvancedReceipt(previous[item.id], item)!;
    return next;
  });
  const [chosen, setChosen] = useState<string | null>(null); const [closed, setClosed] = useState<string[]>([]);
  const queryClient = useQueryClient();
  const [authorityError, setAuthorityError] = useState<MemberError | null>(null);
  const authorityRevision = useRef(0);
  const quarantine = (failure: unknown) => {
    if (!isAuthorityFailure(failure)) return;
    authorityRevision.current++; setAuthorityError(failure); setSelected(null); setInviting(false);
  };
  const controllerRef = useRef<AbortController | null>(null);
  useEffect(() => { const current = new AbortController(); controllerRef.current = current; return () => current.abort(); }, []);
  const storageKey = `membership.pending:${JSON.stringify([scope.expectedUserId, scope.expectedOrganizationId])}`;
  const raw = useSyncExternalStore(subscribe, () => { try { return sessionStorage.getItem(storageKey); } catch { return "unreadable"; } }, noPending);
  const local = useMemo(() => { try { return {items:readPending(raw),error:false}; } catch { return {items:[] as Pending[],error:true}; } }, [raw]);
  const query = useQuery({ queryKey: ["members", scope.expectedUserId, scope.expectedOrganizationId, offset], queryFn: ({ signal }) => getMembers({ ...scope, signal }, offset), gcTime: 0, staleTime: 0, retry: false });
  const ready = query.isSuccess && !query.isFetching && !authorityError;
  const canManage = ready && query.data.canManage;
  const operations = useInfiniteQuery({queryKey:["member-operations",scope.expectedUserId,scope.expectedOrganizationId],initialPageParam:"",getNextPageParam:(page:MemberOperations)=>page.next || undefined,queryFn:async({signal,pageParam})=>{try{const page=await getMemberOperations({...scope,signal},pageParam);if(!signal.aborted)remember(page.items);return page;}catch(failure){if(!signal.aborted)quarantine(failure);throw failure;}},enabled:canManage,gcTime:0,staleTime:0,retry:false});
  const durable = useMemo(() => operations.data?.pages.flatMap(page=>page.items) ?? [], [operations.data]);
  const keys = [...new Set([...local.items.map(item=>item.key),...durable.map(item=>item.id),...Object.keys(receipts)])].filter(key=>!closed.includes(key));
  const activeKey = chosen && keys.includes(chosen) ? chosen : keys[0];
  const pending = local.items.find(item=>item.key===activeKey);
  const operationKey = ["member-operation",scope.expectedUserId,scope.expectedOrganizationId,activeKey];
  const operation = useQuery({ queryKey:operationKey, queryFn: async ({ signal }) => { try { const receipt=await getMemberOperation({ ...scope, signal }, activeKey!);if(!signal.aborted)remember([receipt]);return receipt; } catch (failure) { if (!signal.aborted) quarantine(failure); throw failure; } }, enabled:!!activeKey && !authorityError, gcTime:0,staleTime:0,retry:false });
  const receiptFor = (key:string) => retainAdvancedReceipt(retainAdvancedReceipt(durable.find(item=>item.id===key),receipts[key]),operation.data?.id===key ? operation.data : undefined);
  const currentReceipt = authorityError ? undefined : activeKey ? receiptFor(activeKey) : undefined;
  const directoryComplete = ready && query.data.items.length === query.data.total;
  const formalMemberCount = directoryComplete ? query.data.items.filter(member => member.state === "active").length : null;
  const administratorCount = directoryComplete ? query.data.items.filter(member => member.roles.includes("listingkit_admin")).length : null;
  const inactiveCount = directoryComplete ? query.data.items.filter(member => member.state !== "active").length : null;
  const refreshMembers = async () => {
    const revision = authorityRevision.current;
    setSelected(null); setInviting(false);
    const refreshed = await query.refetch();
    // An older refresh must not undo an authority failure observed in flight.
    if (refreshed.isSuccess && revision === authorityRevision.current) { setAuthorityError(null); setError(""); }
  };
  const persist = (value: Pending | string) => { savePending(sessionStorage,storageKey,value); window.dispatchEvent(new Event("membership-pending")); };
  const run = async (key: string, command?: Pending, verify = false) => {
    const controller = controllerRef.current;
    if (!controller || controller.signal.aborted || busyRef.current || !canManage) return;
    busyRef.current=true;
    setBusy(true); setError("");
    try {
      await queryClient.cancelQueries({queryKey:["member-operation",scope.expectedUserId,scope.expectedOrganizationId,key]});
      const requestScope = { ...scope, signal: controller.signal };
      const result = verify || !command ? await verifyMemberOperation(requestScope, key) : command.kind === "invite" ? await inviteMember(requestScope, key, command.input) : command.kind === "role" ? await changeMemberRole(requestScope, key, command.target, command.input) : await removeMember(requestScope, key, command.target, command.input);
      if (!controller.signal.aborted) { remember([result]); queryClient.setQueryData(["member-operation",scope.expectedUserId,scope.expectedOrganizationId,key],result); void query.refetch(); void operations.refetch(); }
    } catch (failure) { if (!controller.signal.aborted) { quarantine(failure); setError(failure instanceof MemberError ? failure.code : "DEPENDENCY_UNAVAILABLE"); } }
    finally { busyRef.current=false; if (!controller.signal.aborted) setBusy(false); }
  };
  const submit = (command: Pending) => {
    if (busyRef.current || !canManage || local.error) return;
    try { persist(command); setChosen(command.key); setSelected(null); setInviting(false); void run(command.key,command); }
    catch { setError("OPERATION_STORAGE_UNAVAILABLE"); }
  };
  return <div className={styles.page}>
    <div className={styles.toolbar}><p>{ready ? `当前企业 · ${query.data.total} 位成员` : "当前企业成员"}</p><div><Button variant="outline" onClick={() => void refreshMembers()} disabled={busy}>刷新成员</Button>{canManage && <Button disabled={local.error || busy || !query.data.assignableRoles.length} onClick={() => { setSelected(null); setInviting(true); }}>邀请成员</Button>}</div></div>
    {error && !authorityError && <MemberFailure code={error} />}
    {local.error && <MemberFailure code="OPERATION_STORAGE_UNAVAILABLE" />}
    {canManage && <section className={styles.panel} aria-label="待处理成员操作">
      <div className={styles.toolbar}><h2>待处理成员操作</h2><Button variant="outline" disabled={busy || operations.isFetching} onClick={()=>void operations.refetch()}>刷新待处理操作</Button></div>
      <p>结果待核实的操作仍保留原成员或邮箱占用。其他成员的操作可以继续。</p>
      {operations.isError && <p role="alert">待处理列表暂不可用；已保存的操作标识继续保留。</p>}
      {operations.isPending && <p>正在找回待处理操作…</p>}
      <ul className={styles.pendingList}>{keys.map(key=><li key={key}><Button variant={key===activeKey ? "secondary" : "outline"} onClick={()=>setChosen(key)} disabled={busy} aria-pressed={key===activeKey}><span>{receiptFor(key)?.kind === "role" ? "角色调整" : receiptFor(key)?.kind === "remove" ? "移除成员" : "成员操作"}</span><code>{key}</code><span>{terminalReceipt(receiptFor(key)) ? "已有终局回执" : "待核实"}</span></Button></li>)}</ul>
      {operations.isSuccess && keys.length===0 && <p>当前没有待处理操作。</p>}
      {operations.hasNextPage && <Button variant="outline" disabled={busy || operations.isFetching} onClick={()=>void operations.fetchNextPage()}>加载更多待处理操作</Button>}
    </section>}
    {activeKey && <section className={styles.panel} aria-labelledby="pending-title"><h2 id="pending-title">{currentReceipt?.status === "acknowledged" ? "操作已获服务确认" : currentReceipt?.status === "rejected" ? "操作未执行" : "结果待核实"}</h2>
      <p>{currentReceipt?.status === "acknowledged" ? "操作回执与当前成员状态分别显示，请以最新读取的成员资料为准。" : "请保留本次操作标识。核实可继续尚未发送的步骤；已发送的步骤不会再次发送。"}</p>
      <p>当前操作：<span>{activeKey}</span></p>
      {operation.isError && !authorityError && <p role="alert">暂未取得此操作的正式回执，原标识仍保留。</p>}
      {currentReceipt?.userEvidence === "identity_verified" && <p>已核实新用户身份；这不代表验证邮件已送达。</p>}
      {currentReceipt?.observation === "not_visible" && <p>当前读取未看到该成员；这不能单独证明本次移除成功。</p>}
      <div className={styles.actions}>{terminalReceipt(currentReceipt) ? <Button variant="outline" disabled={busy || !canManage} onClick={()=>{try{persist(activeKey);setClosed(previous=>[...previous,activeKey]);setChosen(null);setError("");}catch{setError("OPERATION_STORAGE_UNAVAILABLE");}}}>关闭回执</Button> : <>
        <Button variant="outline" disabled={busy || !canManage} onClick={()=>void run(activeKey,pending,true)}>核实原操作</Button>
        <Button variant="outline" disabled={busy || !canManage} onClick={()=>void run(activeKey,pending)}>继续原操作</Button>
      </>}</div>
    </section>}
    {!ready ? authorityError ? <MemberFailure code={authorityError.code} /> : query.isError ? <MemberFailure code={query.error instanceof MemberError ? query.error.code : "DEPENDENCY_UNAVAILABLE"} /> : <ConsoleState kind="loading" title="正在读取成员">正在确认成员目录与当前权限。</ConsoleState> : <>
      <div className={styles.sectionTabs} role="group" aria-label="成员与权限页面分区"><span aria-current="page">成员管理</span><button type="button" disabled title="当前组织服务未提供角色权限矩阵">角色权限 · 暂不可用</button></div>
      <section className={styles.memberMetrics} aria-label="成员目录摘要"><article><span>正式成员</span><strong>{formalMemberCount ?? "未提供"}</strong><small>{formalMemberCount === null ? "目录分页未覆盖全体成员" : "来自完整成员目录的有效成员"}</small></article><article><span>管理员</span><strong>{administratorCount ?? "未提供"}</strong><small>{administratorCount === null ? "目录分页未覆盖全体成员" : "来自完整成员目录的管理员角色"}</small></article><article><span>邀请中</span><strong>未提供</strong><small>当前 owner 未返回邀请状态</small></article><article><span>已停用</span><strong>{inactiveCount ?? "未提供"}</strong><small>{inactiveCount === null ? "目录分页未覆盖全体成员" : "来自完整成员目录的停用状态"}</small></article></section>
      <div className={styles.memberFilters} aria-label="成员筛选能力"><label>搜索成员<input disabled placeholder="目录服务未提供搜索" /></label><label>角色筛选<select disabled><option>当前目录未提供筛选</option></select></label><label>状态筛选<select disabled><option>当前目录未提供筛选</option></select></label></div>
      <p className={styles.authorityNote}>角色权限详情由组织身份服务管理；本页面只显示 owner 返回的成员角色和可执行管理动作。</p>
      {inviting && canManage && !local.error && <InvitationForm roles={query.data.assignableRoles} onCancel={() => setInviting(false)} onSubmit={input => submit({ key: crypto.randomUUID(), kind: "invite", input })} />}
      {selected && <MemberDetail key={selected} id={selected} scope={scope} roles={query.data.assignableRoles} disabled={busy || local.error} onClose={() => setSelected(null)} onSubmit={submit} onAuthorityFailure={quarantine} />}
      {query.data.items.length === 0 ? <ConsoleState kind="empty" title="当前没有可显示的成员">当前企业的成员目录读取成功。</ConsoleState> : <div className={styles.tableWrap} role="region" aria-label="成员表格，可横向滚动" tabIndex={0}><table className={styles.table}><caption className={styles.caption}>当前企业成员</caption><thead><tr><th>成员</th><th>账号</th><th>角色</th><th>权限范围</th><th>状态</th><th>最近活跃</th><th>操作</th></tr></thead><tbody>{query.data.items.map(member => <tr key={member.id}><td><strong>{member.displayName || member.loginName || member.userId}</strong></td><td>{member.loginName || "未提供"}</td><td>{member.roles.map(value => roleNames[value as MemberRole] ?? value).join("、") || "未授予角色"}</td><td>未提供</td><td>{member.state === "active" ? "有效" : "已停用"}</td><td>未提供</td><td><Button variant="ghost" onClick={() => { setInviting(false); setSelected(member.id); }}>查看详情</Button></td></tr>)}</tbody></table></div>}
      <div className={styles.pagination}><Button variant="outline" disabled={offset === 0 || busy} onClick={() => { setSelected(null); setOffset(value => Math.max(0, value - 20)); }}>上一页</Button><span>第 {Math.floor(offset / 20) + 1} 页</span><Button variant="outline" disabled={offset + 20 >= query.data.total || busy} onClick={() => { setSelected(null); setOffset(value => value + 20); }}>下一页</Button></div>
    </>}
  </div>;
}

function InvitationForm({ roles, onCancel, onSubmit }: { roles: MemberRole[]; onCancel: () => void; onSubmit: (input: z.infer<typeof invitationInput>) => void }) {
  const [invalid, setInvalid] = useState(false);
  return <section className={styles.panel}><h2>邀请新成员</h2><p>创建新的企业成员身份。验证邮件由身份服务发送，已有身份不会被自动关联。</p><form onSubmit={event => { event.preventDefault(); const form = new FormData(event.currentTarget); const parsed = invitationInput.safeParse(Object.fromEntries(form)); setInvalid(!parsed.success); if (parsed.success) onSubmit(parsed.data); }}>{invalid && <p role="alert">请填写有效邮箱和非空姓名，姓名不能包含控制字符。</p>}<div className={styles.formGrid}><label>邮箱<Input name="email" type="email" maxLength={200} required /></label><label>名字<Input name="firstName" maxLength={200} required /></label><label>姓氏<Input name="lastName" maxLength={200} required /></label><label>角色<RoleSelect name="role" roles={roles} /></label></div><div className={styles.actions}><Button type="submit">确认邀请</Button><Button type="button" variant="outline" onClick={onCancel}>取消</Button></div></form></section>;
}

function MemberDetail({ id, scope, roles, disabled, onClose, onSubmit, onAuthorityFailure }: { id: string; scope: MemberScope; roles: MemberRole[]; disabled: boolean; onClose: () => void; onSubmit: (pending: Pending) => void; onAuthorityFailure: (failure: unknown) => void }) {
  const detail = useQuery({ queryKey: ["member-detail", scope.expectedUserId, scope.expectedOrganizationId, id], queryFn: async ({ signal }) => { try { return await getMember({ ...scope, signal }, id); } catch (failure) { if (!signal.aborted) onAuthorityFailure(failure); throw failure; } }, gcTime: 0, staleTime: 0, retry: false });
  const [confirmed, setConfirmed] = useState(false);
  if (detail.isFetching || detail.isPending) return <ConsoleState kind="loading" title="正在读取成员详情" />;
  if (detail.isError || detail.data.items.length !== 1) return <MemberFailure code={detail.error instanceof MemberError ? detail.error.code : "MEMBER_NOT_FOUND"} />;
  const member = detail.data.items[0];
  return <section className={styles.panel}><div className={styles.toolbar}><h2>{member.displayName || member.loginName} · 成员详情</h2><Button variant="outline" onClick={onClose}>关闭详情</Button></div><dl className={styles.facts}><div><dt>成员 ID</dt><dd>{member.userId}</dd></div><div><dt>加入时间</dt><dd>{new Date(member.createdAt).toLocaleString()}</dd></div><div><dt>最近变更</dt><dd>{new Date(member.changedAt).toLocaleString()}</dd></div></dl>
    {member.canChangeRole && <form onSubmit={event => { event.preventDefault(); onSubmit({ key: crypto.randomUUID(), kind: "role", target: id, input: { role: new FormData(event.currentTarget).get("role") as MemberRole, expectedVersion: member.observedVersion } }); }}><label>新的成员角色<RoleSelect name="role" roles={roles} initialRole={member.roles.length === 1 ? member.roles[0] : ""} /></label><Button disabled={disabled} type="submit">保存角色</Button></form>}
    {member.canRemove && <div className={styles.removal}><label><input type="checkbox" checked={confirmed} onChange={event => setConfirmed(event.target.checked)} disabled={disabled} /> 确认移除该成员在当前企业项目中的访问权限</label><Button variant="destructive" disabled={disabled || !confirmed} onClick={() => onSubmit({ key: crypto.randomUUID(), kind: "remove", target: id, input: { expectedVersion: member.observedVersion } })}>移除成员</Button></div>}
    {!member.canChangeRole && !member.canRemove && <p>当前成员仅可查看。</p>}
  </section>;
}
function RoleSelect({ roles, name, initialRole }: { roles: MemberRole[]; name: string; initialRole?: string }) { return <select name={name} className={styles.select} defaultValue={initialRole} required>{initialRole === "" && <option value="" disabled>请选择角色</option>}{roles.map(role => <option key={role} value={role}>{roleNames[role]}</option>)}</select>; }
function MemberFailure({ code }: { code: string }) {
  const messages: Record<string, string> = { AUTHENTICATION_REQUIRED: "登录已失效，请重新登录", IDENTITY_CONTEXT_CHANGED: "登录身份已变化", ORGANIZATION_SELECTION_REQUIRED: "请选择当前企业", ORGANIZATION_CONTEXT_CHANGED: "当前企业已变化", PERMISSION_DENIED: "当前没有成员管理权限", MEMBER_NOT_FOUND: "当前无法读取该成员或操作", MEMBER_OPERATION_CONFLICT: "成员状态或操作已变化，请核实原操作后刷新", DEADLINE_EXCEEDED: "请求超时，写入结果需核实", OPERATION_STORAGE_UNAVAILABLE: "无法保存本次操作标识，请检查浏览器存储", INVALID_UPSTREAM_RESPONSE: "成员服务响应无效" };
  return <ConsoleState kind="error" title={messages[code] ?? "成员服务暂不可用"}>本次未取得完整结果。请确认登录和当前企业后重试；已有操作请沿原标识核实。</ConsoleState>;
}
