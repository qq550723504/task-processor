"use client";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { z } from "zod";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { configurationRequest, ConfigurationError, type ConfigurationScope } from "@/lib/api/agent-configuration";
import { catalogEntrySchema, configReceiptSchema, configVersion } from "@/lib/contracts/agent-configuration";
import { imageTemplateSchema, imageTemplatesPageSchema, imageSetTemplateSchema, imageTemplateInputSchema, carouselTasks, detailTasks, type ImageAgentTemplate, type ImageSetTemplate } from "@/lib/contracts/image-set-configuration";
import { knowledgeId } from "@/lib/api/knowledge";
import { ConsolePage, ConsoleState } from "../console/console-page";
import { ResourceDialog } from "../resources/resource-dialog";
import { useResourcePending } from "../resources/resource-pending";
import { configurationError, useConfigurationRead } from "./agent-configuration-hooks";
import { ImageConfigurationDialog } from "./image-configuration-dialog";
import styles from "./agents.module.css";
import {RecentImageSets} from "./recent-image-sets";

const agentId = "product.image.agent";
const activationNames = { ENABLED: "已启用", DISABLED: "已停用", NOT_ENABLED: "尚未启用" };
const capabilityNames: Record<string, string> = { "image.generate": "图片生成", "product.source-evidence": "商品素材读取", "platform.write": "平台写入" };
const emptyPayload = z.strictObject({});
const defaultPayload = z.strictObject({templateId:knowledgeId.nullable(),revision:configVersion.nullable()}).refine(value => (value.templateId === null) === (value.revision === null));
const intentSchema = z.strictObject({path:z.string().max(200),payload:z.unknown(),method:z.enum(["POST","PUT"]),key:z.uuid(),operation:configReceiptSchema.shape.operation,revision:configVersion.optional(),absent:z.boolean().optional(),templateId:knowledgeId.optional()}).refine(command => {
  switch(command.operation){
    case "enable": return command.path === `${agentId}/enable` && command.method === "POST" && emptyPayload.safeParse(command.payload).success && (!!command.revision !== !!command.absent);
    case "disable": return command.path === `${agentId}/disable` && command.method === "POST" && !!command.revision && emptyPayload.safeParse(command.payload).success;
    case "default": return command.path === `${agentId}/default-template` && command.method === "PUT" && !!command.revision && defaultPayload.safeParse(command.payload).success;
    case "create-template": return command.path === `${agentId}/templates` && command.method === "POST" && imageTemplateInputSchema.safeParse(command.payload).success;
    case "update-template": return !!command.templateId && !!command.revision && command.path === `${agentId}/templates/${command.templateId}` && command.method === "PUT" && imageTemplateInputSchema.safeParse(command.payload).success;
    case "archive-template": return !!command.templateId && !!command.revision && command.path === `${agentId}/templates/${command.templateId}/archive` && command.method === "POST" && emptyPayload.safeParse(command.payload).success;
  }
});
type Intent = z.infer<typeof intentSchema>;
type Draft = { name: string; targetPlatform: ImageAgentTemplate["targetPlatform"]; image: ImageSetTemplate; original?: ImageAgentTemplate };
function initialDraft(): Draft {
  return { name: "", targetPlatform: "product", image: { schema: "image-config-v1", mode: "standard", shareOriginals: true, background: "白色", language: "en", carousel: [{ id: "main-identity", purpose: "product_identity" }], detail: [{ id: "detail-closeup", purpose: "detail_closeup" }] } };
}

export function ImageAgentPage({ scope, organization }: { scope: ConfigurationScope; organization: string }) {
  const context = useWorkbenchContext();
  const pending = useResourcePending({expectedUserId:scope.userId,expectedOrganizationId:scope.organizationId},["agent-configuration",agentId],intentSchema,{storage:"local",maxLength:128*1024});
  const intent = pending.command;
  const [nonce, setNonce] = useState(0), [after, setAfter] = useState("");
  const [draft, setDraft] = useState<Draft | null>(null), [configure, setConfigure] = useState(false), [disable, setDisable] = useState(false);
  const [chosen, setChosen] = useState<ImageAgentTemplate | null>(null), [history, setHistory] = useState("");
  const [busy, setBusy] = useState(false), [failure, setFailure] = useState<unknown>(null), [formError, setFormError] = useState(""), [message, setMessage] = useState("");
  const active = useRef(true), flight = useRef(false), abort = useRef<AbortController | null>(null);
  useEffect(() => {
    active.current = true;
    return () => { active.current = false; abort.current?.abort(); };
  }, []);
  const registerSwitchGuard = context.registerOrganizationSwitchGuard;
  useEffect(() => registerSwitchGuard(() => !flight.current && !intent && !pending.error), [registerSwitchGuard,intent,pending.error]);
  const detail = useConfigurationRead(scope, agentId, catalogEntrySchema, nonce);
  const entry = detail.data;
  // A full image template may contain 64 KiB; one row fits the 128 KiB response cap.
  const templates = useConfigurationRead(scope, `${agentId}/templates?pageSize=1${after ? `&cursor=${encodeURIComponent(after)}` : ""}`, imageTemplatesPageSchema, nonce, !!entry && entry.agent.activation !== "NOT_ENABLED");
  const defaultRef = entry?.agent.defaultTemplate;
  const defaultTemplate = useConfigurationRead(scope, defaultRef ? `${agentId}/templates/${defaultRef.templateId}/revisions/${defaultRef.revision}` : "", imageTemplateSchema, nonce, !!defaultRef);
  const historic = useConfigurationRead(scope, chosen && configVersion.safeParse(history).success ? `${agentId}/templates/${chosen.templateId}/revisions/${history}` : "", imageTemplateSchema, nonce, !!chosen && configVersion.safeParse(history).success);
  const locked = busy || !!intent || !pending.ready;

  async function send(command: Intent, verification = false) {
    if (flight.current || !active.current || !pending.ready) return;
    flight.current = true;
    setBusy(true); setFailure(null); setMessage("");
    const controller = new AbortController(); abort.current = controller;
    const headers = new Headers({ "Content-Type": "application/json", "Idempotency-Key": command.key });
    if (command.absent) headers.set("If-None-Match", "*");
    else if (command.revision) headers.set("If-Match", `"${command.revision}"`);
    try {
      pending.persist(command);
      await configurationRequest(scope, command.path, configReceiptSchema.refine(receipt => receipt.agentId === agentId && receipt.operation === command.operation && (!command.templateId || receipt.templateId === command.templateId)), { method: command.method, headers, body: JSON.stringify(command.payload), signal: controller.signal });
      if (!active.current) return;
      pending.clear(command); setDraft(null); setDisable(false); setChosen(null); setConfigure(false);
      setMessage("操作已保存，正在读取当前企业配置。"); setNonce(value => value + 1);
    } catch (error) {
      if (!active.current) return;
      setFailure(error);
      if (!verification && error instanceof ConfigurationError && error.code !== "OUTCOME_UNKNOWN" && [400,403,404,409,412,422].includes(error.status)) pending.clear(command);
    } finally {
      flight.current = false;
      if (active.current) setBusy(false);
    }
  }
  function mutate(command: Omit<Intent, "key">) {
    if (locked) return;
    void send({ ...command, key: crypto.randomUUID() });
  }
  function save() {
    if (!draft || locked) return;
    const name = draft.name.trim();
    if (!name || [...name].length > 120 || new TextEncoder().encode(name).length > 512 || /[\u0000-\u001f\u007f]/.test(name)) { setFormError("请填写 1–120 字的模板名称。"); return; }
    const image = imageSetTemplateSchema.safeParse(draft.image);
    if (!image.success) { setFormError(image.error.issues[0]?.message ?? "请检查图片配置。"); return; }
    setFormError("");
    mutate({ path: draft.original ? `${agentId}/templates/${draft.original.templateId}` : `${agentId}/templates`, payload: { name, targetPlatform: draft.targetPlatform, image: image.data }, method: draft.original ? "PUT" : "POST", operation: draft.original ? "update-template" : "create-template", revision: draft.original?.revision, templateId: draft.original?.templateId });
  }

  return <ConsolePage title="商品图片智能体" description={`当前企业：${organization} · 整套主图与详情图配置`} breadcrumbs={[{ label: "我的智能体", href: "/workbench/agents/mine" }, { label: "商品图片智能体" }]} actions={<Button variant="outline" disabled={busy} onClick={() => setNonce(value => value + 1)}>刷新配置</Button>}>
    {message && <p role="status">{message}</p>}
    {intent ? <ConsoleState kind="error" title={failure ? configurationError(failure) : "原操作尚未核实，请沿用原请求编号。"}><Button disabled={busy} onClick={() => void send(intent,true)}>核实原操作</Button></ConsoleState> : failure ? <ConsoleState kind="error" title={configurationError(failure)} /> : null}
    {pending.error && <ConsoleState kind="unavailable" title="无法读取原操作记录，请恢复浏览器存储后重试。" />}
    {detail.error ? <ConsoleState kind="error" title={configurationError(detail.error)} /> : !entry ? <ConsoleState kind="loading" title="正在读取图片智能体配置" /> : <>
      <Card className={styles.detailHead}>
        <div><h2>{entry.name}</h2><p>{entry.description}</p><p>{activationNames[entry.agent.activation]} · 配置版本 {entry.agent.revision || "尚未建立"}</p></div>
        {entry.canConfigure && (entry.agent.activation === "ENABLED" ? <Button variant="outline" disabled={locked} onClick={() => setDisable(true)}>停用智能体</Button> : <Button disabled={locked} onClick={() => mutate({ path: `${agentId}/enable`, payload: {}, method: "POST", operation: "enable", revision: entry.agent.revision || undefined, absent: entry.agent.activation === "NOT_ENABLED" })}>启用到当前企业</Button>)}
      </Card>
      <Card className={styles.panel}><h2>当前能力</h2>{entry.capabilities.filter(cap => cap.support === "REQUIRED").map(cap => <div className={styles.capability} key={cap.id}><div><h3>{capabilityNames[cap.id] ?? cap.id}</h3><p>{cap.reason}</p></div><span>{cap.readiness === "AVAILABLE" ? "可用" : cap.readiness === "NEEDS_CONFIGURATION" ? "需要配置" : cap.readiness === "REQUIRES_AUTHORIZATION" ? "需要授权" : "当前不可用"}</span></div>)}<p>使用时先选择真实原图与平台类目，再核对整套计划和点数。主图与详情图分别生成，共用素材时仅共用原图。</p>{entry.canUse && <Link href="/workbench/supply/acquisition">进入采集商品选择素材 →</Link>}</Card>
      <Card className={styles.panel}><h2>企业默认模板</h2>{defaultRef ? <><p>{defaultTemplate.data ? `${defaultTemplate.data.name} · v${defaultTemplate.data.version}` : defaultTemplate.error ? configurationError(defaultTemplate.error) : "正在读取默认模板"}</p>{entry.canConfigure && <Button variant="outline" disabled={locked} onClick={() => mutate({ path: `${agentId}/default-template`, payload: { templateId: null, revision: null }, method: "PUT", operation: "default", revision: entry.agent.revision })}>清除默认模板</Button>}</> : <p>尚未设置默认模板，生成时需要显式选择模板。</p>}</Card>
      <Card className={styles.panel}><div className={styles.cardActions}><h2>图片模板</h2>{entry.canConfigure && entry.agent.activation !== "NOT_ENABLED" && <Button disabled={locked || !!draft} onClick={() => { setDraft(initialDraft()); setFormError(""); }}>新建图片模板</Button>}</div>
        {templates.error ? <p role="alert">{configurationError(templates.error)}</p> : templates.data?.items.length ? <div className={styles.templateList}>{templates.data.items.map(template => <section key={template.templateId} className={styles.templateRow}><div><h3>{template.name} · v{template.version}</h3><p>{template.targetPlatform === "product" ? "通用商品素材" : template.targetPlatform.toUpperCase()} · 主图 {template.image.carousel.length} 张 · 详情图 {template.image.detail.length} 张 · {template.lifecycle === "ACTIVE" ? "可选用" : "已归档"}</p><p>{template.image.shareOriginals ? "共用原始素材，两组独立生成" : "两组分别选择素材并生成"}</p></div><div className={styles.actions}>
          <Button variant="outline" onClick={() => { setChosen(template); setHistory(template.version); }}>查看版本</Button>
          {entry.canConfigure && template.lifecycle === "ACTIVE" && <>
            <Button variant="outline" disabled={locked || !!draft} onClick={() => { setDraft({ name: template.name, targetPlatform: template.targetPlatform, image: structuredClone(template.image), original: template }); setFormError(""); }}>编辑</Button>
            <Button disabled={locked} onClick={() => mutate({ path: `${agentId}/default-template`, payload: { templateId: template.templateId, revision: template.version }, method: "PUT", operation: "default", revision: entry.agent.revision })}>设为默认 v{template.version}</Button>
            <Button variant="outline" disabled={locked} onClick={() => mutate({ path: `${agentId}/templates/${template.templateId}/archive`, payload: {}, method: "POST", operation: "archive-template", revision: template.revision, templateId: template.templateId })}>归档</Button>
          </>}
        </div></section>)}</div> : <p>{entry.agent.activation === "NOT_ENABLED" ? "请先启用到当前企业。" : templates.data ? "当前企业尚无图片模板。" : "正在读取模板。"}</p>}
        {(after || templates.data?.nextCursor) && <nav className={styles.actions} aria-label="图片模板分页">{after && <Button variant="outline" onClick={() => setAfter("")}>返回第一页</Button>}{templates.data?.nextCursor && <Button variant="outline" onClick={() => setAfter(templates.data!.nextCursor)}>下一页</Button>}</nav>}
      </Card>
    </>}
    {entry?.canReadRuns&&<RecentImageSets key={`${scope.userId}:${scope.organizationId}`} scope={scope} nonce={nonce}/>}
    {draft && <Card className={styles.panel}><h2>{draft.original ? `编辑模板 · 当前 v${draft.original.version}` : "新建图片模板"}</h2><label>模板名称<Input aria-label="模板名称" value={draft.name} disabled={locked} onChange={event => setDraft({ ...draft, name: event.target.value })} /></label><label>素材平台<Select aria-label="素材平台" value={draft.targetPlatform} disabled={locked} onChange={event => setDraft({ ...draft, targetPlatform: event.target.value as Draft["targetPlatform"] })}><option value="product">通用商品素材</option><option value="shein">SHEIN</option>{["temu", "amazon"].includes(draft.targetPlatform) && <option value={draft.targetPlatform}>{draft.targetPlatform.toUpperCase()} · 当前未开放执行</option>}</Select></label><p>主图 {draft.image.carousel.length} 张 · 详情图 {draft.image.detail.length} 张 · {draft.image.background} · {draft.image.language}</p><div className={styles.actions}><Button variant="outline" disabled={locked} onClick={() => setConfigure(true)}>配置整套图片</Button><Button variant="outline" disabled={locked} onClick={() => setDraft(null)}>取消编辑</Button><Button disabled={locked} onClick={save}>保存图片模板</Button></div>{formError && <p role="alert">{formError}</p>}</Card>}
    {configure && draft && <ImageConfigurationDialog initial={draft.image} locked={locked} onClose={() => setConfigure(false)} onSave={image => { setDraft({ ...draft, image }); setConfigure(false); }} />}
    {disable && entry && <ResourceDialog title="停用商品图片智能体" locked={locked} onClose={() => setDisable(false)}><p>停用后不能确认新的生成计划。已准入的任务仍按原预算完成，图片资产与已有回执保留。</p><div className={styles.actions}><Button variant="outline" disabled={locked} onClick={() => setDisable(false)}>取消</Button><Button disabled={locked} onClick={() => mutate({ path: `${agentId}/disable`, payload: {}, method: "POST", operation: "disable", revision: entry.agent.revision })}>确认停用</Button></div></ResourceDialog>}
    {chosen && <Card className={styles.panel}><div className={styles.cardActions}><h2>{chosen.name} · 不可变模板版本</h2><Button variant="outline" onClick={() => setChosen(null)}>关闭版本</Button></div><label>版本号<Input aria-label="图片模板版本号" value={history} onChange={event => setHistory(event.target.value)} /></label>{historic.error ? <p role="alert">{configurationError(historic.error)}</p> : historic.data ? <><p>v{historic.data.version} · {historic.data.targetPlatform} · {historic.data.image.background} · {historic.data.image.language}</p><p>{historic.data.image.shareOriginals ? "共用原始素材，两组仍独立生成" : "两组分别选择原始素材"}</p>{(["carousel", "detail"] as const).map(group => <div key={group}><h3>{group === "carousel" ? "主图任务" : "详情图任务"}</h3><ol>{historic.data!.image[group].map(task => <li key={task.id}>{[...carouselTasks, ...detailTasks].find(item => item.purpose === task.purpose)?.label ?? "自定义任务"}{task.brief ? `：${task.brief}` : ""}</li>)}</ol></div>)}</> : <p>输入有效版本号以读取此版本。</p>}</Card>}
  </ConsolePage>;
}
