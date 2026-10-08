"use client";

import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";

import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";

import { changeMemberRole, getMember, getMemberOperation, getMemberOperations, getMembers, MemberError, MemberFilters, memberListFilterSchema, MemberOperation, MemberOperations, MemberRole, MemberScope, removeMember, verifyMemberOperation } from "@/lib/api/members";
import { Pending, readPending, retainAdvancedReceipt, savePending, terminalReceipt } from "./member-pending";
import { ConsoleState } from "../console/console-page";
import { AccountShell } from "./account-shell";
import styles from "./members.module.css";
import {InvitationsPanel} from "./invitations";
import { AccountDialog } from "./account-dialog";
import { RolePermissions } from "./role-permissions";
import { EnterpriseRole } from "@/lib/api/enterprise-roles";
import {MemberStats,PermissionScope,RecentMemberActivity} from "./member-facts";

const displayRoles = (definitions: EnterpriseRole[]) => Object.fromEntries(definitions.map(role => [role.id, role.name]));
const subscribe = (notify: () => void) => { window.addEventListener("membership-pending", notify); window.addEventListener("storage", notify); return () => { window.removeEventListener("membership-pending", notify); window.removeEventListener("storage", notify); }; };
const noPending = () => null;
const isAuthorityFailure = (failure: unknown): failure is MemberError => failure instanceof MemberError && (failure.status === 401 || failure.status === 403 || ["IDENTITY_CONTEXT_CHANGED", "ORGANIZATION_CONTEXT_CHANGED", "ORGANIZATION_SELECTION_REQUIRED"].includes(failure.code));

export function MembersPage({ expectedUserId }: { expectedUserId: string }) {
  const context = useWorkbenchContext(); const [leaving, setLeaving] = useState(false);
  useEffect(() => { const click = (event: MouseEvent) => { const link = event.target instanceof Element ? event.target.closest("a") : null; if (link && new URL(link.href, location.href).pathname === "/api/zitadel-auth/logout") setLeaving(true); }; document.addEventListener("click", click, true); return () => document.removeEventListener("click", click, true); }, []);
  const org = context.effectiveOrganization;
  const scope = JSON.stringify([expectedUserId, context.user?.id, org?.id, context.roles, context.permissions, context.isLoading, context.error?.code, context.blockingError?.code]);
  let content;
  if (leaving || context.user?.id !== expectedUserId) content = <MemberFailure code="AUTHENTICATION_REQUIRED" />;
  else if (context.isSwitching || context.isLoading) content = <ConsoleState kind="loading" title="正在确认当前企业">旧成员资料已清除。</ConsoleState>;
  else if (!org || context.selectionRequired || context.error || context.blockingError) content = <MemberFailure code={context.blockingError?.code ?? context.error?.code ?? "ORGANIZATION_SELECTION_REQUIRED"} />;
  else return <ScopedMembers key={scope} scope={{ expectedUserId, expectedOrganizationId: org.id }} />;
  return <AccountShell pathname="/workbench/account/organization/members" title="成员与权限">{content}</AccountShell>;
}

function ScopedMembers({ scope }: { scope:MemberScope }) {
  const [tab,setTab]=useState<"roles"|"members">("members"),[inviteRequest,setInviteRequest]=useState(0),[canInvite,setCanInvite]=useState(false);

  const [memberTab,setMemberTab]=useState<"formal"|"inviting">("formal");
  const [offset, setOffset] = useState(0); const [selected, setSelected] = useState<string | null>(null);
  const [filters, setFilters] = useState<MemberFilters>({q:"",role:"",state:""});
  const [search, setSearch] = useState(""); const [filterError, setFilterError] = useState("");
  const filtered = !!(filters.q || filters.role || filters.state);
  const applyFilters = (next:MemberFilters) => { setSelected(null);setOffset(0);setFilters(next);setFilterError(""); };
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
    authorityRevision.current++; setAuthorityError(failure); setSelected(null); };
  const controllerRef = useRef<AbortController | null>(null);
  useEffect(() => { const current = new AbortController(); controllerRef.current = current; return () => current.abort(); }, []);
  const storageKey = `membership.pending:${JSON.stringify([scope.expectedUserId, scope.expectedOrganizationId])}`;
  const raw = useSyncExternalStore(subscribe, () => { try { return sessionStorage.getItem(storageKey); } catch { return "unreadable"; } }, noPending);
  const local = useMemo(() => { try { return {items:readPending(raw),error:false}; } catch { return {items:[] as Pending[],error:true}; } }, [raw]);
  const query = useQuery({ queryKey: ["members", scope.expectedUserId, scope.expectedOrganizationId, filters, offset], queryFn: async ({ signal }) => { try { return await getMembers({ ...scope, signal }, offset, filters); } catch(failure) { if(!signal.aborted)quarantine(failure);throw failure; } }, gcTime: 0, staleTime: 0, retry: false });
  const ready = query.isSuccess && !query.isFetching && !authorityError;
  const canManage = ready && query.data.canManage;
  const roleNames=displayRoles(query.data?.roleDefinitions ?? []);
  const operations = useInfiniteQuery({queryKey:["member-operations",scope.expectedUserId,scope.expectedOrganizationId],initialPageParam:"",getNextPageParam:(page:MemberOperations)=>page.next || undefined,queryFn:async({signal,pageParam})=>{try{const page=await getMemberOperations({...scope,signal},pageParam);if(!signal.aborted)remember(page.items);return page;}catch(failure){if(!signal.aborted)quarantine(failure);throw failure;}},enabled:canManage,gcTime:0,staleTime:0,retry:false});
  const durable = useMemo(() => operations.data?.pages.flatMap(page=>page.items) ?? [], [operations.data]);
  const keys = [...new Set([...local.items.map(item=>item.key),...durable.map(item=>item.id),...Object.keys(receipts)])].filter(key=>!closed.includes(key));
  const activeKey = chosen && keys.includes(chosen) ? chosen : keys[0];
  const pending = local.items.find(item=>item.key===activeKey);
  const operationKey = ["member-operation",scope.expectedUserId,scope.expectedOrganizationId,activeKey];
  const operation = useQuery({ queryKey:operationKey, queryFn: async ({ signal }) => { try { const receipt=await getMemberOperation({ ...scope, signal }, activeKey!);if(!signal.aborted)remember([receipt]);return receipt; } catch (failure) { if (!signal.aborted) quarantine(failure); throw failure; } }, enabled:!!activeKey && !authorityError, gcTime:0,staleTime:0,retry:false });
  const receiptFor = (key:string) => retainAdvancedReceipt(retainAdvancedReceipt(durable.find(item=>item.id===key),receipts[key]),operation.data?.id===key ? operation.data : undefined);
  const currentReceipt = authorityError ? undefined : activeKey ? receiptFor(activeKey) : undefined;
  const refreshMembers = async () => {
    const revision = authorityRevision.current;
    setSelected(null); const refreshed = await query.refetch();
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
      const result = verify || !command ? await verifyMemberOperation(requestScope, key) : command.kind === "invite" ? await verifyMemberOperation(requestScope, key) : command.kind === "role" ? await changeMemberRole(requestScope, key, command.target, command.input) : await removeMember(requestScope, key, command.target, command.input);
      if (!controller.signal.aborted) { remember([result]); queryClient.setQueryData(["member-operation",scope.expectedUserId,scope.expectedOrganizationId,key],result); void query.refetch(); void operations.refetch(); }
    } catch (failure) { if (!controller.signal.aborted) { quarantine(failure); setError(failure instanceof MemberError ? failure.code : "DEPENDENCY_UNAVAILABLE"); } }
    finally { busyRef.current=false; if (!controller.signal.aborted) setBusy(false); }
  };
  const submit = (command: Pending) => {
    if (busyRef.current || !canManage || local.error) return;
    try { persist(command); setChosen(command.key); setSelected(null); void run(command.key,command); }
    catch { setError("OPERATION_STORAGE_UNAVAILABLE"); }
  };
  return <AccountShell pathname="/workbench/account/organization/members" title="成员与权限" description={tab === "roles" ? "系统仅保留管理员角色；其他角色由企业自定义。角色只控制模块是否可用。" : "邀请成员、分配角色并管理业务权限；资源与消费上限统一在「资源与额度」中分配。"} actions={tab==="members" && canManage ? <Button disabled={!canInvite || busy} onClick={()=>setInviteRequest(value=>value+1)}>邀请成员</Button> : undefined}><div className={styles.page}>
    <div className={styles.sectionTabs} role="tablist" aria-label="成员与权限"><button role="tab" aria-selected={tab==="roles"} onClick={()=>setTab("roles")}>角色权限</button><button role="tab" aria-selected={tab==="members"} onClick={()=>setTab("members")}>成员管理</button></div>
    {tab==="roles" && !authorityError ? <RolePermissions scope={scope} onChanged={()=>void query.refetch()} onAuthorityFailure={quarantine}/> : null}
    <div hidden={tab!=="members"}>
    {error && !authorityError && <MemberFailure code={error} />}
    {local.error && <MemberFailure code="OPERATION_STORAGE_UNAVAILABLE" />}
    {canManage && (keys.length > 0 || operations.isError) && <section className={styles.panel} aria-label="待处理成员操作">
      <div className={styles.toolbar}><h2>待处理成员操作</h2><Button variant="outline" disabled={busy || operations.isFetching} onClick={()=>void operations.refetch()}>刷新待处理操作</Button></div>
      <p>结果待核实的操作仍保留原成员或邮箱占用。其他成员的操作可以继续。</p>
      {operations.isError && <p role="alert">待处理列表暂不可用；已保存的操作标识继续保留。</p>}
      {operations.isPending && <p>正在找回待处理操作…</p>}
      <ul className={styles.pendingList}>{keys.map(key=>{
        const receipt = receiptFor(key);
        return <li key={key}><Button variant={key===activeKey ? "secondary" : "outline"} onClick={()=>setChosen(key)} disabled={busy} aria-pressed={key===activeKey}><span>{receipt?.kind === "role" ? "角色调整" : receipt?.kind === "remove" ? "移除成员" : "成员操作"}</span><code>{key}</code><span>{!receipt ? "尚未读取回执" : terminalReceipt(receipt) ? "已有终局回执" : receipt.status === "unknown" ? "待核实" : "待继续"}</span></Button></li>;
      })}</ul>
      {operations.isSuccess && keys.length===0 && <p>当前没有待处理操作。</p>}
      {operations.hasNextPage && <Button variant="outline" disabled={busy || operations.isFetching} onClick={()=>void operations.fetchNextPage()}>加载更多待处理操作</Button>}
    </section>}
    {activeKey && <section className={styles.panel} aria-labelledby="pending-title"><h2 id="pending-title">{!currentReceipt ? "尚未读取正式回执" : currentReceipt.status === "acknowledged" ? "操作已获服务确认" : currentReceipt.status === "rejected" ? "操作未执行" : currentReceipt.status === "unknown" ? "结果待核实" : "操作待继续"}</h2>
      <p>{!currentReceipt ? "当前仅保留操作标识；读取正式回执前不判断操作结果。" : currentReceipt.status === "acknowledged" ? "操作回执与当前成员状态分别显示，请以最新读取的成员资料为准。" : "请保留本次操作标识。核实可继续尚未发送的步骤；已发送的步骤不会再次发送。"}</p>
      <p>当前操作：<span>{activeKey}</span></p>
      {operation.isError && !authorityError && <p role="alert">暂未取得此操作的正式回执，原标识仍保留。</p>}
      {currentReceipt?.userEvidence === "identity_verified" && <p>已核实新用户身份；这不代表验证邮件已送达。</p>}
      {currentReceipt?.observation === "not_visible" && <p>当前读取未看到该成员；这不能单独证明本次移除成功。</p>}
      <div className={styles.actions}>{terminalReceipt(currentReceipt) ? <Button variant="outline" disabled={busy || !canManage} onClick={()=>{try{persist(activeKey);setClosed(previous=>[...previous,activeKey]);setChosen(null);setError("");}catch{setError("OPERATION_STORAGE_UNAVAILABLE");}}}>关闭回执</Button> : <>
        <Button variant="outline" disabled={busy || !canManage} onClick={()=>void run(activeKey,pending,true)}>核实原操作</Button>
        <Button variant="outline" disabled={busy || !canManage} onClick={()=>void run(activeKey,pending)}>继续原操作</Button>
      </>}</div>
    </section>}
    <p className="sr-only" role="status">{ready ? `${filtered ? "筛选结果" : "当前企业"} · ${query.data.total} 位成员` : "当前企业成员"}</p>
    {!authorityError && <MemberStats scope={scope} />}
    <div className={styles.memberListToolbar}>{!authorityError && <div className={styles.memberTabs} role="tablist" aria-label="成员列表"><button role="tab" aria-selected={memberTab==="formal"} onClick={()=>setMemberTab("formal")}>正式成员</button><button role="tab" disabled={!canManage} aria-selected={memberTab==="inviting"} onClick={()=>setMemberTab("inviting")}>邀请中</button></div>}<Button className={styles.refreshMembers} variant="ghost" onClick={()=>void refreshMembers()} disabled={busy}>刷新成员</Button></div>
    <div hidden={!!authorityError}>
      {canManage && <InvitationsPanel scope={scope} assignableRoles={query.data.assignableRoles} roleDefinitions={query.data.roleDefinitions} showList={memberTab==="inviting"} inviteRequest={inviteRequest} onInviteAvailability={setCanInvite} onChanged={()=>{void query.refetch();void queryClient.invalidateQueries({queryKey:["invitation-summary",scope.expectedUserId,scope.expectedOrganizationId]});}} onAuthorityFailure={quarantine}/>}

    <div hidden={memberTab!=="formal"}><form className={styles.memberFilters} aria-label="成员筛选" onSubmit={event=>{event.preventDefault();const next=memberListFilterSchema.safeParse({...filters,q:search});if(next.success){applyFilters(next.data);setSearch(next.data.q);}else setFilterError("搜索内容过长或包含无效字符，请缩短后重试。");}}>
      <div className={styles.searchField}><label htmlFor="member-search">搜索成员</label><div className={styles.searchInput}><input id="member-search" value={search} onChange={event=>setSearch(event.target.value)} placeholder="搜索姓名或登录账号" aria-describedby="member-search-hint member-search-error" aria-invalid={!!filterError} disabled={busy || !!authorityError}/><Button type="submit" variant="outline" disabled={busy || !!authorityError}>搜索</Button></div></div>
      <label>角色筛选<select value={filters.role} onChange={event=>applyFilters({...filters,role:event.target.value as MemberFilters["role"]})} disabled={busy || !!authorityError}><option value="">全部角色</option>{Object.entries(roleNames).map(([value,label])=><option key={value} value={value}>{label}</option>)}</select></label>
      <label>状态筛选<select value={filters.state} onChange={event=>applyFilters({...filters,state:event.target.value as MemberFilters["state"]})} disabled={busy || !!authorityError}><option value="">全部状态</option><option value="active">有效</option><option value="inactive">已停用</option></select></label>
    </form>
    <div className={styles.filterHelp}><p id="member-search-hint">搜索当前企业完整目录的姓名和登录账号；手机号或邮箱作为登录账号时可匹配。角色、状态可组合筛选，成员统计仍为企业全量。</p><Button variant="ghost" disabled={busy || !!authorityError} onClick={()=>{setSearch("");applyFilters({q:"",role:"",state:""});}}>清除筛选</Button></div>
    <p id="member-search-error" className={styles.authorityNote} role={filterError ? "alert" : undefined}>{filterError}</p></div>
    </div>
    {!ready ? authorityError ? <MemberFailure code={authorityError.code} /> : query.isError ? <MemberFailure code={query.error instanceof MemberError ? query.error.code : "DEPENDENCY_UNAVAILABLE"} /> : <ConsoleState kind="loading" title="正在读取成员">正在确认成员目录与当前权限。</ConsoleState> : <><div hidden={memberTab!=="formal"}>
      {selected && <MemberDetail key={selected} id={selected} scope={scope} roles={query.data.assignableRoles} definitions={query.data.roleDefinitions} disabled={busy || local.error} onClose={() => setSelected(null)} onSubmit={submit} onAuthorityFailure={quarantine} />}
      {query.data.items.length === 0 ? <ConsoleState kind="empty" title={filtered ? "没有匹配的成员" : "当前没有可显示的成员"}>{filtered ? "当前企业目录读取成功，请调整或清除筛选条件。" : "当前企业的成员目录读取成功。"}</ConsoleState> : <div className={styles.tableWrap} role="region" aria-label="成员表格，可横向滚动" tabIndex={0}><table className={styles.table}><caption className={styles.caption}>当前企业成员</caption><thead><tr><th>成员</th><th>账号</th><th>角色</th><th>权限范围</th><th>状态</th><th>最近活跃</th><th>操作</th></tr></thead><tbody>{query.data.items.map(member => <tr key={member.id}><td><strong>{member.displayName || member.loginName || member.userId}</strong></td><td>{member.loginName || "未提供"}</td><td>{member.roles.map(value => roleNames[value as MemberRole] ?? value).join("、") || "未授予角色"}</td><td><PermissionScope permissions={member.permissions}/></td><td>{member.state === "active" ? "有效" : "已停用"}</td><td><RecentMemberActivity scope={scope} userId={member.userId}/></td><td><Button variant="ghost" onClick={() => { setSelected(member.id); }}>查看详情</Button></td></tr>)}</tbody></table></div>}
      <div className={styles.pagination}><Button variant="outline" disabled={offset === 0 || busy} onClick={() => { setSelected(null); setOffset(value => Math.max(0, value - 20)); }}>上一页</Button><span>第 {Math.floor(offset / 20) + 1} 页</span><Button variant="outline" disabled={offset + 20 >= query.data.total || busy} onClick={() => { setSelected(null); setOffset(value => value + 20); }}>下一页</Button></div>
      </div>
    </>}
    </div>
  </div></AccountShell>;
}

function MemberDetail({ id, scope, roles, definitions, disabled, onClose, onSubmit, onAuthorityFailure }: { id: string; scope: MemberScope; roles: MemberRole[]; definitions: EnterpriseRole[]; disabled: boolean; onClose: () => void; onSubmit: (pending: Pending) => void; onAuthorityFailure: (failure: unknown) => void }) {
  const detail = useQuery({ queryKey: ["member-detail", scope.expectedUserId, scope.expectedOrganizationId, id], queryFn: async ({ signal }) => { try { return await getMember({ ...scope, signal }, id); } catch (failure) { if (!signal.aborted) onAuthorityFailure(failure); throw failure; } }, gcTime: 0, staleTime: 0, retry: false });
  const [confirmed, setConfirmed] = useState(false);
  if (detail.isFetching || detail.isPending) return <ConsoleState kind="loading" title="正在读取成员详情" />;
  if (detail.isError || detail.data.items.length !== 1) return <MemberFailure code={detail.error instanceof MemberError ? detail.error.code : "MEMBER_NOT_FOUND"} />;
  const member = detail.data.items[0];
  return <AccountDialog title="成员详情" drawer onClose={onClose}><section className={styles.detail}><h3>{member.displayName || member.loginName}</h3><p>{member.loginName || "未提供账号"}</p><p>{member.roles.map(role=>displayRoles(definitions)[role] ?? role).join("、")}</p><PermissionScope permissions={member.permissions}/><RecentMemberActivity scope={scope} userId={member.userId}/><dl className={styles.facts}><div><dt>成员 ID</dt><dd>{member.userId}</dd></div><div><dt>加入时间</dt><dd>{new Date(member.createdAt).toLocaleString()}</dd></div><div><dt>最近变更</dt><dd>{new Date(member.changedAt).toLocaleString()}</dd></div></dl>
    {member.canChangeRole && <form onSubmit={event => { event.preventDefault(); onSubmit({ key: crypto.randomUUID(), kind: "role", target: id, input: { role: new FormData(event.currentTarget).get("role") as MemberRole, expectedVersion: member.observedVersion } }); }}><label>新的成员角色<RoleSelect name="role" roles={roles} definitions={definitions} initialRole={member.roles.length === 1 ? member.roles[0] : ""} /></label><Button disabled={disabled} type="submit">保存角色</Button></form>}
    {member.canRemove && <div className={styles.removal}><label><input type="checkbox" checked={confirmed} onChange={event => setConfirmed(event.target.checked)} disabled={disabled} /> 确认移除该成员在当前企业项目中的访问权限</label><Button variant="destructive" disabled={disabled || !confirmed} onClick={() => onSubmit({ key: crypto.randomUUID(), kind: "remove", target: id, input: { expectedVersion: member.observedVersion } })}>移除成员</Button></div>}
    {!member.canChangeRole && !member.canRemove && <p>当前成员仅可查看。</p>}
  </section></AccountDialog>;
}
function RoleSelect({ roles, definitions, name, initialRole }: { roles: MemberRole[]; definitions: EnterpriseRole[]; name: string; initialRole?: string }) { return <select name={name} className={styles.select} defaultValue={initialRole} required>{initialRole === "" && <option value="" disabled>请选择角色</option>}{roles.map(role => <option key={role} value={role}>{displayRoles(definitions)[role] ?? role}</option>)}</select>; }
function MemberFailure({ code }: { code: string }) {
  const messages: Record<string, string> = { AUTHENTICATION_REQUIRED: "登录已失效，请重新登录", IDENTITY_CONTEXT_CHANGED: "登录身份已变化", ORGANIZATION_SELECTION_REQUIRED: "请选择当前企业", ORGANIZATION_CONTEXT_CHANGED: "当前企业已变化", PERMISSION_DENIED: "当前没有成员管理权限", MEMBER_NOT_FOUND: "当前无法读取该成员或操作", MEMBER_OPERATION_CONFLICT: "成员状态或操作已变化，请核实原操作后刷新", DEADLINE_EXCEEDED: "请求超时，写入结果需核实", OPERATION_STORAGE_UNAVAILABLE: "无法保存本次操作标识，请检查浏览器存储", INVALID_UPSTREAM_RESPONSE: "成员服务响应无效" };
  return <ConsoleState kind="error" title={messages[code] ?? "成员服务暂不可用"}>本次未取得完整结果。请确认登录和当前企业后重试；已有操作请沿原标识核实。</ConsoleState>;
}
