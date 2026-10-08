"use client";
import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { MemberError, MemberScope } from "@/lib/api/members";
import { createEnterpriseRole, createRoleInput, EnterpriseRole, EnterpriseRoles, getEnterpriseRoles, saveEnterpriseRole, saveRoleInput } from "@/lib/api/enterprise-roles";
import { AccountDialog } from "./account-dialog";
import { z } from "zod";
import { enterpriseRoleKey } from "@/lib/api/enterprise-role-schema";
import styles from "./roles.module.css";

type Intent = { key: string; id: string; input: { name?: string; modules: string[]; expectedVersion?: number } };
export function RolePermissions({ scope, onChanged, onAuthorityFailure }: { scope: MemberScope; onChanged: () => void; onAuthorityFailure: (failure: unknown) => void }) {
  const query = useQuery({ queryKey: ["enterprise-roles", scope.expectedUserId, scope.expectedOrganizationId], queryFn: async ({ signal }) => { try { return await getEnterpriseRoles({ ...scope, signal }); } catch (failure) { if (!signal.aborted) onAuthorityFailure(failure); throw failure; } }, gcTime: 0, staleTime: 0, retry: false });
  const [selected, setSelected] = useState("listingkit_admin"), [creating, setCreating] = useState(false), [busy, setBusy] = useState(false), [error, setError] = useState(""), [intent, setIntent] = useState<Intent | null>(null), [storageError, setStorageError] = useState(false);
  const key = `enterprise-role.intent:${JSON.stringify([scope.expectedUserId, scope.expectedOrganizationId])}`;
  const controller = useRef<AbortController | null>(null), active = useRef(false);
  useEffect(() => {
    const c = new AbortController(); controller.current = c;
    queueMicrotask(() => { if(c.signal.aborted) return; try { const raw = sessionStorage.getItem(key); if (raw) { const value = JSON.parse(raw); if (!z.uuid().safeParse(value.key).success || (value.id !== "" && !enterpriseRoleKey.safeParse(value.id).success)) throw new Error("invalid intent"); const parsed = value.id ? saveRoleInput.parse(value.input) : createRoleInput.parse(value.input); setIntent({ key: value.key, id: value.id, input: parsed }); } } catch { setStorageError(true); } });
    return () => c.abort();
  }, [key]);
  async function run(value: Intent) {
    const c = controller.current; if (!c || c.signal.aborted || active.current || !query.data?.canManage || storageError) return;
    active.current = true; setBusy(true); setError("");
    try {
      sessionStorage.setItem(key, JSON.stringify(value)); setIntent(value);
      const request = { ...scope, signal: c.signal };
      const result = value.id ? await saveEnterpriseRole(request, value.key, value.id, saveRoleInput.parse(value.input)) : await createEnterpriseRole(request, value.key, createRoleInput.parse(value.input));
      if (!c.signal.aborted) { sessionStorage.removeItem(key); setIntent(null); setSelected(result.role.id); setCreating(false); await query.refetch(); onChanged(); }
    } catch (failure) {
      if (!c.signal.aborted) {
        setCreating(false);
        onAuthorityFailure(failure);
        if (failure instanceof MemberError && [400, 403, 409].includes(failure.status)) { sessionStorage.removeItem(key); setIntent(null); void query.refetch(); }
        setError(failure instanceof MemberError && failure.status === 409 ? "角色已变化或名称已存在，请重新读取后修改。" : "本次尚未取得保存结果；请核实原操作，避免重复创建。");
      }
    } finally { active.current = false; if (!c.signal.aborted) setBusy(false); }
  }
  if (query.isPending) return <p role="status">正在读取当前企业的角色权限…</p>;
  if (query.isError) return <div role="alert">角色权限暂不可用。<Button variant="outline" onClick={() => void query.refetch()}>重新读取角色</Button></div>;
  const data = query.data, role = data.items.find(r => r.id === selected) ?? data.items[0];
  return <>
    {(error || storageError) && <p className={styles.notice} role="alert">{storageError ? "无法读取已保存的操作标识，暂不能更改角色。" : error}</p>}
    {intent && <div className={styles.notice}><p>有一项角色保存结果待确认。原操作标识：{intent.key}</p><Button disabled={busy || !data.canManage} variant="outline" onClick={() => void run(intent)}>核实原保存</Button></div>}
    <div className={styles.layout}><aside className={styles.roleList} aria-label="企业角色"><h2>角色</h2><p>系统角色</p>{data.items.filter(r => r.system).map(r => <RoleButton key={r.id} role={r} selected={role.id === r.id} disabled={busy || !!intent} choose={() => setSelected(r.id)} />)}<p>自定义角色</p>{data.items.filter(r => !r.system).map(r => <RoleButton key={r.id} role={r} selected={role.id === r.id} disabled={busy || !!intent} choose={() => setSelected(r.id)} />)}{data.items.length === 1 && <small>尚未创建自定义角色</small>}<Button className={styles.newRole} variant="outline" disabled={!data.canCreate || busy || !!intent || storageError} onClick={() => setCreating(true)}>＋ 新建角色</Button>{!data.canCreate && data.canManage && <small>当前企业没有可用的新角色槽位</small>}</aside>
      <RoleEditor key={`${role.id}:${role.version}`} role={role} data={data} disabled={busy || !!intent || storageError || query.isFetching} onSave={modules => void run({ key: crypto.randomUUID(), id: role.id, input: { modules, expectedVersion: role.version } })} />
    </div>
    {creating && <AccountDialog title="新建角色" onClose={() => { if (!busy) setCreating(false); }}><NewRole data={data} disabled={busy || !!intent} cancel={() => setCreating(false)} submit={(name, modules) => void run({ key: crypto.randomUUID(), id: "", input: { name, modules } })} /></AccountDialog>}
  </>;
}
function RoleButton({ role, selected, disabled, choose }: { role: EnterpriseRole; selected: boolean; disabled: boolean; choose: () => void }) { return <button type="button" className={styles.roleButton} aria-pressed={selected} disabled={disabled} onClick={choose}>{role.name}{role.system && <span> · 系统</span>}</button>; }
function RoleEditor({ role, data, disabled, onSave }: { role: EnterpriseRole; data: EnterpriseRoles; disabled: boolean; onSave: (modules: string[]) => void }) {
  const [modules, setModules] = useState(role.modules), changed = [...modules].sort().join() !== [...role.modules].sort().join();
  return <section className={styles.configuration} aria-label="角色模块权限"><header><div><h2>{role.name}</h2><p>{role.system ? "管理员拥有企业管理权限，系统角色不可修改。" : "角色只控制模块是否可用，不包含企业管理与财务权限。"}</p></div>{!role.system && <Button disabled={disabled || !data.canManage || !changed} onClick={() => onSave(modules)}>保存</Button>}</header><p>展开一级菜单，添加或移除子菜单，完成后点击保存。</p><ModuleTree catalog={data.catalog} modules={modules} disabled={disabled || role.system || !data.canManage} change={setModules} /></section>;
}
function NewRole({ data, disabled, submit, cancel }: { data: EnterpriseRoles; disabled: boolean; submit: (name: string, modules: string[]) => void; cancel: () => void }) {
  const [name, setName] = useState(""), [modules, setModules] = useState<string[]>([]);
  return <form onSubmit={e => { e.preventDefault(); if (createRoleInput.safeParse({ name, modules }).success) submit(name, modules); }}><label className={styles.nameField}>角色名称<input autoComplete="off" value={name} disabled={disabled} onChange={e => setName([...e.target.value].slice(0, 40).join(""))} placeholder="请输入角色名称" required /><small>{[...name].length}/40</small></label><h3>模块权限</h3><div className={styles.modalTree}><ModuleTree catalog={data.catalog} modules={modules} disabled={disabled} change={setModules} checkboxes /></div><footer><Button variant="outline" type="button" disabled={disabled} onClick={cancel}>取消</Button><Button type="submit" disabled={disabled || !createRoleInput.safeParse({ name, modules }).success}>创建</Button></footer></form>;
}
function ModuleTree({ catalog, modules, disabled, change, checkboxes = false }: { catalog: EnterpriseRoles["catalog"]; modules: string[]; disabled: boolean; change: (modules: string[]) => void; checkboxes?: boolean }) {
  const groups = [...new Set(catalog.map(m => m.group))];
  const toggle = (id: string) => change(modules.includes(id) ? modules.filter(m => m !== id) : [...modules, id]);
  return <div className={checkboxes ? styles.checkTree : styles.moduleTree}>{groups.map(group => <details key={group} className={styles.group}><summary><strong>{group}</strong><span>{catalog.filter(m => m.group === group && modules.includes(m.id)).length}项</span></summary><div>{catalog.filter(m => m.group === group).map(module => <div className={styles.module} key={module.id}>{checkboxes ? <label><input type="checkbox" checked={modules.includes(module.id)} disabled={disabled || !module.available} onChange={() => toggle(module.id)} />{module.label}</label> : <span>{module.label}</span>}{!module.available ? <small>尚未开放</small> : !checkboxes && <Button variant="outline" size="sm" disabled={disabled} aria-label={`${modules.includes(module.id) ? "移除" : "添加"}${module.label}`} onClick={() => toggle(module.id)}>{modules.includes(module.id) ? "移除" : "添加"}</Button>}</div>)}</div></details>)}</div>;
}
