"use client";

import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { z } from "zod";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { changeMemberRole, getMember, getMemberOperation, getMembers, invitationInput, inviteMember, MemberError, memberId, MemberOperation, MemberRole, MemberScope, removeInput, removeMember, roleInput, verifyMemberOperation } from "@/lib/api/members";
import { ConsoleState } from "../console/console-page";
import { AccountShell } from "./account-shell";
import styles from "./members.module.css";

const pendingSchema = z.discriminatedUnion("kind", [
  z.object({ key: z.string().uuid(), kind: z.literal("invite"), input: invitationInput }).strict(),
  z.object({ key: z.string().uuid(), kind: z.literal("role"), target: memberId, input: roleInput }).strict(),
  z.object({ key: z.string().uuid(), kind: z.literal("remove"), target: memberId, input: removeInput }).strict(),
]);
type Pending = z.infer<typeof pendingSchema>;
const roleNames: Record<MemberRole, string> = { listingkit_viewer: "只读成员", listingkit_operator: "操作成员", listingkit_admin: "企业管理员" };
const subscribe = (notify: () => void) => { window.addEventListener("membership-pending", notify); window.addEventListener("storage", notify); return () => { window.removeEventListener("membership-pending", notify); window.removeEventListener("storage", notify); }; };
const noPending = () => null;

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
  return <AccountShell pathname="/workbench/account/organization/members" title="成员管理" description="查看当前企业的成员与访问角色">{content}</AccountShell>;
}

function ScopedMembers({ scope }: { scope: MemberScope }) {
  const [offset, setOffset] = useState(0); const [selected, setSelected] = useState<string | null>(null); const [inviting, setInviting] = useState(false);
  const [busy, setBusy] = useState(false); const [error, setError] = useState(""); const [receipt, setReceipt] = useState<MemberOperation | null>(null);
  const controllerRef = useRef<AbortController | null>(null);
  useEffect(() => { const current = new AbortController(); controllerRef.current = current; return () => current.abort(); }, []);
  const storageKey = `membership.pending:${JSON.stringify([scope.expectedUserId, scope.expectedOrganizationId])}`;
  const raw = useSyncExternalStore(subscribe, () => { try { return sessionStorage.getItem(storageKey); } catch { return null; } }, noPending);
  const pending = useMemo(() => { try { if (!raw || raw.length > 4096) return null; const parsed = pendingSchema.safeParse(JSON.parse(raw)); return parsed.success ? parsed.data : null; } catch { return null; } }, [raw]);
  const query = useQuery({ queryKey: ["members", scope.expectedUserId, scope.expectedOrganizationId, offset], queryFn: ({ signal }) => getMembers({ ...scope, signal }, offset), gcTime: 0, staleTime: 0, retry: false });
  const operation = useQuery({ queryKey: ["member-operation", scope.expectedUserId, scope.expectedOrganizationId, pending?.key], queryFn: ({ signal }) => getMemberOperation({ ...scope, signal }, pending!.key), enabled: !!pending, gcTime: 0, staleTime: 0, retry: false });
  const currentReceipt = receipt?.id === pending?.key ? receipt : operation.data;
  const ready = query.isSuccess && !query.isFetching;
  const canManage = ready && query.data.canManage;
  const persist = (value: Pending | null) => { if (value) sessionStorage.setItem(storageKey, JSON.stringify(value)); else sessionStorage.removeItem(storageKey); window.dispatchEvent(new Event("membership-pending")); };
  const run = async (command: Pending, verify = false) => {
    const controller = controllerRef.current;
    if (!controller || controller.signal.aborted) return;
    setBusy(true); setError("");
    try {
      const requestScope = { ...scope, signal: controller.signal };
      const result = verify ? await verifyMemberOperation(requestScope, command.key) : command.kind === "invite" ? await inviteMember(requestScope, command.key, command.input) : command.kind === "role" ? await changeMemberRole(requestScope, command.key, command.target, command.input) : await removeMember(requestScope, command.key, command.target, command.input);
      if (!controller.signal.aborted) { setReceipt(result); void query.refetch(); }
    } catch (failure) { if (!controller.signal.aborted) setError(failure instanceof MemberError ? failure.code : "DEPENDENCY_UNAVAILABLE"); }
    finally { if (!controller.signal.aborted) setBusy(false); }
  };
  const submit = (command: Pending) => {
    if (pending || busy || !canManage) return;
    try { persist(command); setReceipt(null); setSelected(null); setInviting(false); void run(command); }
    catch { setError("OPERATION_STORAGE_UNAVAILABLE"); }
  };
  return <div className={styles.page}>
    <div className={styles.toolbar}><p>{ready ? `当前企业 · ${query.data.total} 位成员` : "当前企业成员"}</p><div><Button variant="outline" onClick={() => { setSelected(null); setInviting(false); void query.refetch(); }} disabled={busy}>刷新成员</Button>{canManage && <Button disabled={!!pending || busy || !query.data.assignableRoles.length} onClick={() => { setSelected(null); setInviting(true); }}>邀请成员</Button>}</div></div>
    {error && <MemberFailure code={error} />}
    {pending && <section className={styles.panel} aria-labelledby="pending-title"><h2 id="pending-title">{currentReceipt?.status === "acknowledged" ? "操作已获服务确认" : currentReceipt?.status === "rejected" ? "操作未执行" : "结果待核实"}</h2><p>{currentReceipt?.status === "acknowledged" ? "操作回执与当前成员状态分别显示，请以最新读取的成员资料为准。" : "请保留本次操作标识。核实可继续尚未发送的步骤；已发送的步骤不会再次发送。"}</p><code>{pending.key}</code>{currentReceipt?.userEvidence === "identity_verified" && <p>已核实新用户身份；这不代表验证邮件已送达。</p>}{currentReceipt?.observation === "not_visible" && <p>当前读取未看到该成员；这不能单独证明本次移除成功。</p>}<div className={styles.actions}>{currentReceipt && ["acknowledged", "rejected"].includes(currentReceipt.status) ? <Button variant="outline" onClick={() => { persist(null); setReceipt(null); setError(""); }}>关闭回执</Button> : <><Button variant="outline" disabled={busy || !canManage} onClick={() => void run(pending, true)}>核实原操作</Button><Button variant="outline" disabled={busy || !canManage} onClick={() => void run(pending)}>继续原操作</Button></>}</div></section>}
    {!ready ? query.isError ? <MemberFailure code={query.error instanceof MemberError ? query.error.code : "DEPENDENCY_UNAVAILABLE"} /> : <ConsoleState kind="loading" title="正在读取成员">正在确认成员目录与当前权限。</ConsoleState> : <>
      {inviting && canManage && !pending && <InvitationForm roles={query.data.assignableRoles} onCancel={() => setInviting(false)} onSubmit={input => submit({ key: crypto.randomUUID(), kind: "invite", input })} />}
      {selected && <MemberDetail key={selected} id={selected} scope={scope} roles={query.data.assignableRoles} disabled={busy || !!pending} onClose={() => setSelected(null)} onSubmit={submit} />}
      {query.data.items.length === 0 ? <ConsoleState kind="empty" title="当前没有可显示的成员">当前企业的成员目录读取成功。</ConsoleState> : <div className={styles.tableWrap} role="region" aria-label="成员表格，可横向滚动" tabIndex={0}><table className={styles.table}><caption className={styles.caption}>当前企业成员</caption><thead><tr><th>成员</th><th>角色</th><th>状态</th><th>操作</th></tr></thead><tbody>{query.data.items.map(member => <tr key={member.id}><td><strong>{member.displayName || member.loginName || member.userId}</strong><span>{member.loginName}</span></td><td>{member.roles.map(value => roleNames[value as MemberRole] ?? value).join("、") || "未授予角色"}</td><td>{member.state === "active" ? "有效" : "已停用"}</td><td><Button variant="ghost" onClick={() => { setInviting(false); setSelected(member.id); }}>查看详情</Button></td></tr>)}</tbody></table></div>}
      <div className={styles.pagination}><Button variant="outline" disabled={offset === 0 || busy} onClick={() => { setSelected(null); setOffset(value => Math.max(0, value - 20)); }}>上一页</Button><span>第 {Math.floor(offset / 20) + 1} 页</span><Button variant="outline" disabled={offset + 20 >= query.data.total || busy} onClick={() => { setSelected(null); setOffset(value => value + 20); }}>下一页</Button></div>
    </>}
  </div>;
}

function InvitationForm({ roles, onCancel, onSubmit }: { roles: MemberRole[]; onCancel: () => void; onSubmit: (input: z.infer<typeof invitationInput>) => void }) {
  const [invalid, setInvalid] = useState(false);
  return <section className={styles.panel}><h2>邀请新成员</h2><p>创建新的企业成员身份。验证邮件由身份服务发送，已有身份不会被自动关联。</p><form onSubmit={event => { event.preventDefault(); const form = new FormData(event.currentTarget); const parsed = invitationInput.safeParse(Object.fromEntries(form)); setInvalid(!parsed.success); if (parsed.success) onSubmit(parsed.data); }}>{invalid && <p role="alert">请填写有效邮箱和非空姓名，姓名不能包含控制字符。</p>}<div className={styles.formGrid}><label>邮箱<Input name="email" type="email" maxLength={200} required /></label><label>名字<Input name="firstName" maxLength={200} required /></label><label>姓氏<Input name="lastName" maxLength={200} required /></label><label>角色<RoleSelect name="role" roles={roles} /></label></div><div className={styles.actions}><Button type="submit">确认邀请</Button><Button type="button" variant="outline" onClick={onCancel}>取消</Button></div></form></section>;
}

function MemberDetail({ id, scope, roles, disabled, onClose, onSubmit }: { id: string; scope: MemberScope; roles: MemberRole[]; disabled: boolean; onClose: () => void; onSubmit: (pending: Pending) => void }) {
  const detail = useQuery({ queryKey: ["member-detail", scope.expectedUserId, scope.expectedOrganizationId, id], queryFn: ({ signal }) => getMember({ ...scope, signal }, id), gcTime: 0, staleTime: 0, retry: false });
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
