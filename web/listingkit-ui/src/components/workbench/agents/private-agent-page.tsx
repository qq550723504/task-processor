"use client";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { z } from "zod";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ConsolePage, ConsoleState } from "../console/console-page";
import { useResourcePending } from "../resources/resource-pending";
import { CustomizationError, type CustomScope } from "@/lib/api/agent-customization";
import { privateAgentRequest, privateDelivery, privatePage, qualityInput, qualityPage, qualityPending, qualityRun, type QualityRun } from "@/lib/api/private-agent";
import styles from "./agents.module.css";
import qualityStyles from "./private-agent.module.css";
const limitation = "确定性资料检查：检查必需资料缺失、重复或不同值的规格、尺寸单位及重复描述。未调用模型，不推断商品事实，不判断行业合规；报告仅供人工核实。";
const href = (id: string) => "/workbench/agents/mine/private/" + id;
function errorText(e: unknown) { const code = e instanceof CustomizationError ? e.code : ""; return ({ CUSTOMIZATION_NOT_FOUND: "当前企业未获授权使用此智能体。", CUSTOMIZATION_FORBIDDEN: "当前身份没有操作权限。", PERMISSION_DENIED: "当前身份没有操作权限。", OUTCOME_UNKNOWN: "结果尚未确认，请核实同一次检查。", CUSTOMIZATION_CONFLICT: "原操作内容不一致，请保留原记录并联系平台。", CUSTOMIZATION_INVALID: "请检查资料长度与格式；总输入最多16 KiB。", ORGANIZATION_CONTEXT_CHANGED: "当前企业已变化，请重新确认。", IDENTITY_CONTEXT_CHANGED: "登录身份已变化，请重新确认。" } as Record<string, string>)[code] ?? "暂时无法读取或保存，请重新确认当前权限后再试。"; }
function usePrivateRead<T>(scope: CustomScope, path: string, schema: z.ZodType<T>, nonce = 0) {
  const [state, setState] = useState<{ key: string; data?: T; error?: unknown ;}>({ key: "" });
  const { userId, organizationId } = scope, key = path + ":" + nonce;
  useEffect(() => {
const c = new AbortController(), timer = setTimeout(() => c.abort(), 45000); let alive = true;
    void privateAgentRequest({ userId, organizationId }, path, schema, { signal: c.signal }).then(data => { if (alive) setState({ key, data }); }).catch(error => { if (alive) setState({ key, error }); });
    return () => { alive = false; clearTimeout(timer); c.abort(); };
  }, [userId, organizationId, path, schema, key]);
  return state.key === key ? state : { key };
}
export function PrivateAgentCards({ scope }: { scope: CustomScope ;}) {
  const [cursor, setCursor] = useState(""), [nonce, setNonce] = useState(0), ctx = useWorkbenchContext();
  const result = usePrivateRead(scope, cursor ? "?cursor=" + cursor : "", privatePage, nonce);
  if (!ctx.permissions.includes("workbench.agent.read")) return null;
  return <section className={qualityStyles.section} aria-label="企业私有智能体"><h2>企业私有智能体</h2><p>仅当前企业可见，来自平台专员的实际定制交付。</p>
    {result.error ? <ConsoleState kind="unavailable" title="私有智能体暂不可用"><p>{errorText(result.error)}</p><Button variant="outline" onClick={() => setNonce(v => v + 1)}>重新读取私有智能体</Button></ConsoleState> : !result.data ? <ConsoleState kind="loading" title="正在读取私有智能体" /> : !result.data.items.length ? <ConsoleState kind="empty" title="当前企业暂无已交付的私有智能体"><Link href="/workbench/agents/custom">了解智能体定制 →</Link></ConsoleState> : <div className={styles.mineCards}>{result.data.items.map(v => <Card key={v.id} className={styles.agentCard}><div className={styles.cardActions}><h2>{v.name}</h2><span className={styles.status} data-active="true">企业专属 · 可用</span></div><p>检查手动输入的商品资料，生成可保存的质检报告。确定性规则版本 {v.version}。</p><div className={styles.cardActions}><span className={styles.support}>无外部写入 · 不调用模型</span><Button asChild><Link href={href(v.id)}>进入使用</Link></Button></div></Card>)}</div>}
    {result.data?.nextCursor ? <Button variant="outline" onClick={() => setCursor(result.data!.nextCursor)}>更多私有智能体</Button> : null}{cursor ? <Button variant="outline" onClick={() => setCursor("")}>返回首页</Button> : null}
  </section>;
}
export function PrivateAgentPage({ id }: { id: string ;}) {
  const c = useWorkbenchContext();
  if (c.isLoading || c.isSwitching) return <ConsoleState kind="loading" title="正在确认当前企业" />;
  if (c.error || c.blockingError || !c.user || !c.effectiveOrganization) return <ConsoleState kind="unavailable" title="请先确认登录身份与当前企业" />;
  if (!c.permissions.includes("workbench.agent.read")) return <ConsoleState kind="unavailable" title="当前身份没有智能体读取权限" />;
  const scope = { userId: c.user.id, organizationId: c.effectiveOrganization.id };
  return <ScopedPrivate key={[scope.userId, scope.organizationId, id, c.permissions.join("|")].join(":")} scope={scope} id={id} organization={c.effectiveOrganization.name} canUse={c.permissions.includes("workbench.agent.use")} />;
}
function ScopedPrivate({ scope, id, organization, canUse }: { scope: CustomScope; id: string; organization: string; canUse: boolean ;}) {
  const [nonce, setNonce] = useState(0), [cursor, setCursor] = useState(""), [selected, setSelected] = useState<QualityRun | null>(null);
  const delivery = usePrivateRead(scope, "/" + id, privateDelivery, nonce);
  const reports = usePrivateRead(scope, "/" + id + "/reports" + (cursor ? "?cursor=" + cursor : ""), qualityPage, nonce);
  return <ConsolePage title="商品资料质检" description={`企业私有智能体 · 当前企业：${organization}`} breadcrumbs={[{ label: "智能市场" }, { label: "我的智能体", href: "/workbench/agents/mine" }, { label: "商品资料质检" }]} actions={<Link href="/workbench/agents/mine">返回我的智能体 →</Link>}>
    {delivery.error ? <ConsoleState kind="unavailable" title="此智能体在当前企业不可用"><p>{errorText(delivery.error)}</p><Button onClick={() => setNonce(v => v + 1)}>重新读取</Button></ConsoleState> : !delivery.data ? <ConsoleState kind="loading" title="正在读取私有交付" /> : <>
      <Card className={styles.detailHead}><div><h2>{delivery.data.name}</h2><p>{limitation}</p><p>交付版本 {delivery.data.version} · {new Date(delivery.data.createdAt).toLocaleString("zh-CN")}</p></div></Card>
      <div className={qualityStyles.grid}><QualityForm scope={scope} id={id} canUse={canUse} saved={run => { setSelected(run); setCursor(""); setNonce(v => v + 1); }} /><Card className={styles.panel}><h2>检查范围</h2><p>必需资料：名称、材质、尺寸、规格、描述。</p><p>规格中同名不同值会提示核对变体；不会擅自选择其中一个值。</p><p>输入和报告对当前企业有读取权限的成员可见。不会修改商品或发送到外部系统。</p><p>未发现问题也需要人工核实商品真实性与平台要求。</p></Card></div>
      {selected ? <Report run={selected} /> : null}
      <Card className={styles.panel}><h2>已保存报告</h2>{reports.error ? <p role="alert">{errorText(reports.error)}</p> : !reports.data ? <p>正在读取报告…</p> : !reports.data.items.length ? <p>尚未运行资料检查。</p> : <ul className={qualityStyles.history}>{reports.data.items.map(v => <li key={v.id}><div><strong>{v.input.name || "未提供名称"}</strong><p>{new Date(v.createdAt).toLocaleString("zh-CN")} · {v.report.findings.length} 项提示</p></div><Button variant="outline" onClick={() => setSelected(v)}>查看报告</Button></li>)}</ul>}<div className={styles.actions}><Button variant="outline" onClick={() => setNonce(v => v + 1)}>刷新报告</Button>{reports.data?.nextCursor ? <Button variant="outline" onClick={() => setCursor(reports.data!.nextCursor)}>更多报告</Button> : null}{cursor ? <Button variant="outline" onClick={() => setCursor("")}>报告首页</Button> : null}</div></Card>
    </>}
  </ConsolePage>;
}
function QualityForm({ scope, id, canUse, saved }: { scope: CustomScope; id: string; canUse: boolean; saved: (v: QualityRun) => void ;}) {
  const pending = useResourcePending({ expectedUserId: scope.userId, expectedOrganizationId: scope.organizationId }, ["private-quality", id], qualityPending, { storage: "session", maxLength: 131072 });
  const [busy, setBusy] = useState(false), [message, setMessage] = useState(""), [specs, setSpecs] = useState([{ name: "", value: "" }]);
  const live = useRef(true), flight = useRef(false), abort = useRef<AbortController | null>(null), ctx = useWorkbenchContext();
  const locked = busy || !pending.ready || !!pending.command;
  useEffect(() => { live.current = true; return () => { live.current = false; abort.current?.abort(); }; }, []);
  const guard = ctx.registerOrganizationSwitchGuard;
  useEffect(() => guard?.(() => !locked && !flight.current), [guard, locked]);
  async function execute(command: z.infer<typeof qualityPending>) {
    if (flight.current || !pending.ready || !canUse || command.deliveryId !== id) return;
    flight.current = true; setBusy(true); setMessage(""); const c = new AbortController(); abort.current = c; const timer = setTimeout(() => c.abort(), 45000);
    try {
      pending.persist(command);
      const run = await privateAgentRequest(scope, "/" + id + "/reports", qualityRun, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": command.key }, body: JSON.stringify(command.input), signal: c.signal });
      if (!live.current) return; pending.clear(command); setMessage("报告已保存。"); saved(run);
    } catch (e) {
if (!live.current) return; setMessage(errorText(e)); if (e instanceof CustomizationError && ((e.status === 400 && e.code === "CUSTOMIZATION_INVALID") || (e.status === 404 && e.code === "CUSTOMIZATION_NOT_FOUND"))) pending.clear(command);
    } finally { clearTimeout(timer); flight.current = false; if (live.current) setBusy(false); }
  }
  function submit(e: React.FormEvent<HTMLFormElement>) {
e.preventDefault(); if (locked || flight.current || !canUse) return; const f = new FormData(e.currentTarget), get = (key: string) => String(f.get(key) ?? "").trim();
    const input = qualityInput.safeParse({ name: get("name"), material: get("material"), dimensions: get("dimensions"), description: get("description"), specifications: specs.map(s => ({ name: s.name.trim(), value: s.value.trim() })).filter(s => s.name || s.value) });
    if (!input.success || new TextEncoder().encode(JSON.stringify(input.success ? input.data : {})).length > 16384) { setMessage("请提供至少一项资料，并确认文字长度、规格数量和总输入16 KiB上限。"); return; }
    void execute({ key: crypto.randomUUID(), deliveryId: id, input: input.data });
  }
  return <Card className={`${styles.panel} ${qualityStyles.form}`}><h2>输入商品资料</h2><p>可留空缺失字段，检查会提示补充；至少提供一项资料。</p>{!canUse ? <p>当前身份可以读取报告，没有执行权限。</p> : null}{pending.error ? <p role="alert">原操作记录无法读取，请保留浏览器数据并联系平台。</p> : null}{message ? <p role="status">{message}</p> : null}{pending.command ? <div role="status"><p>有一次检查尚未确认，核实会复用原资料和请求编号。</p><Button disabled={busy || !canUse} variant="outline" onClick={() => { if (pending.command) void execute(pending.command); }}>核实同一次检查</Button></div> : null}
    <form onSubmit={submit}><fieldset disabled={locked || !canUse}><label>商品名称<Input name="name" maxLength={120} /></label><label>材质<Input name="material" maxLength={240} /></label><label>尺寸<Input name="dimensions" maxLength={240} placeholder="例如：20 × 10 × 5 cm" /></label><label>商品描述<textarea name="description" maxLength={4000} rows={5} /></label><fieldset><legend>规格</legend>{specs.map((s, i) => <div key={i} className={qualityStyles.spec}><Input aria-label={`规格名称 ${i + 1}`} maxLength={80} placeholder="规格名称" value={s.name} onChange={e => setSpecs(v => v.map((s, n) => n === i ? { ...s, name: e.target.value } : s))} /><Input aria-label={`规格值 ${i + 1}`} maxLength={240} placeholder="规格值" value={s.value} onChange={e => setSpecs(v => v.map((s, n) => n === i ? { ...s, value: e.target.value } : s))} /><Button type="button" variant="outline" onClick={() => setSpecs(v => v.filter((_, n) => n !== i))}>删除</Button></div>)}<Button type="button" variant="outline" disabled={specs.length >= 20} onClick={() => setSpecs(v => [...v, { name: "", value: "" }])}>添加规格</Button></fieldset><Button type="submit">{busy ? "正在检查并保存…" : "检查并保存报告"}</Button></fieldset></form>
  </Card>;
}
function Report({ run }: { run: QualityRun ;}) {
  return <Card className={`${styles.panel} ${qualityStyles.report}`} aria-label="质检报告"><h2>质检报告 · {run.input.name || "未提供名称"}</h2><p>{run.report.summary}</p><p>报告编号 {run.id} · 版本 {run.version} · {new Date(run.createdAt).toLocaleString("zh-CN")}</p><p>执行成员：{run.actorId}</p><dl><dt>材质</dt><dd>{run.input.material || "未提供"}</dd><dt>尺寸</dt><dd>{run.input.dimensions || "未提供"}</dd><dt>描述</dt><dd>{run.input.description || "未提供"}</dd><dt>规格</dt><dd>{run.input.specifications.length ? run.input.specifications.map((v, i) => <p key={i}>{v.name || "缺少名称"}：{v.value || "缺少值"}</p>) : "未提供"}</dd></dl><ol>{run.report.findings.map((v, i) => <li key={i}><strong>{v.message}</strong><p>{v.suggestion}</p></li>)}</ol><p>{limitation}</p></Card>;
}
