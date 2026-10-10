"use client";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ConsolePage, ConsoleState } from "../console/console-page";
import { customizationRequest, CustomizationError, customInput, customPage, customDetail, customReceipt, customUpdate, customStages, stageLabels, type CustomScope, type CustomRequest, type CustomDetail } from "@/lib/api/agent-customization";
import styles from "./customization.module.css";
import { useResourcePending } from "../resources/resource-pending";
import { customizationPending, freezeCustomization, restoreCustomization, forgetCustomization, type CustomIntent as Intent } from "./pending";
type Mode = "intro" | "new" | "progress" | "admin";
const base = "/workbench/agents/custom";
const directions = [["PRODUCT_SUPPLY", "商品与供应链"], ["STORE_OPERATIONS", "店铺运营"], ["DATA_ANALYSIS", "数据分析"], ["OTHER", "其他"]] as const;
const stepDescriptions = ["描述业务场景与目标", "专员联系沟通与评估", "线下确认方案与费用", "开发、测试并跟进进度", "交付说明与使用指导"];
const date = (v: string) => new Date(v).toLocaleString("zh-CN");
function errorText(error: unknown) {
    const code = error instanceof CustomizationError ? error.code : "";
    return ({ OUTCOME_UNKNOWN: "操作结果尚未确认，请核实同一次操作。", CUSTOMIZATION_REVISION_MISMATCH: "进度已被更新，请重新读取后确认。", CUSTOMIZATION_CONFLICT: "同一次操作的内容不一致，请保留原操作并联系平台。", CUSTOMIZATION_INVALID: "请检查必填项、附件与进度前置条件。", CUSTOMIZATION_FORBIDDEN: "当前身份没有这项操作权限。", PERMISSION_DENIED: "当前身份没有这项操作权限。", CUSTOMIZATION_NOT_FOUND: "需求不存在或不属于当前企业。", ORGANIZATION_CONTEXT_CHANGED: "企业已切换，请重新确认当前企业。", IDENTITY_CONTEXT_CHANGED: "登录身份已变化，请重新登录。" } as Record<string, string>)[code] ?? "暂时无法读取或保存定制需求，请稍后重试。";
}
export function CustomizationPage({ mode }: {
    mode: Mode;
}) {
    const c = useWorkbenchContext();
    if (c.isLoading || c.isSwitching)
        return <ConsoleState kind="loading" title="正在确认身份与企业"/>;
    if (!c.user || c.error || c.blockingError)
        return <ConsoleState kind="unavailable" title="请先确认登录身份与当前企业"/>;
    if (mode !== "admin" && !c.effectiveOrganization)
        return <ConsoleState kind="unavailable" title="请先选择企业"/>;
    const canRead = mode === "admin" || c.permissions.includes("workbench.agent.read");
    if (!canRead || mode === "new" && !c.permissions.includes("workbench.agent.use"))
        return <ConsoleState kind="unavailable" title="当前身份没有智能体定制权限"/>;
    const scope = { userId: c.user.id, organizationId: mode === "admin" ? "" : c.effectiveOrganization!.id, admin: mode === "admin" };
    return <ScopedPage key={[mode, scope.userId, scope.organizationId, c.permissions.join("|")].join(":")} mode={mode} scope={scope} organizationName={c.effectiveOrganization?.name ?? ""}/>;
}
function useCommands(scope: CustomScope) {
    const context = useWorkbenchContext(), mounted = useRef(true), running = useRef(false), controllers = useRef(new Set<AbortController>());
    const pending = useResourcePending({ expectedUserId: scope.userId, expectedOrganizationId: scope.organizationId }, ["agent-customization", scope.admin ? "platform" : "enterprise"], customizationPending, { storage: "session", maxLength: 300000 });
    const intent = restoreCustomization(pending.storageKey, pending.command), [busy, setBusy] = useState(false), [message, setMessage] = useState("");
    useEffect(() => { mounted.current = true; const active = controllers.current; return () => { mounted.current = false; active.forEach(c => c.abort()); }; }, []);
    const guard = context.registerOrganizationSwitchGuard;
    const locked = !pending.ready || !!pending.command || busy;
    useEffect(() => guard?.(() => !locked && !running.current), [guard, locked]);
    async function execute(i: Intent) {
        if (running.current || !pending.ready)
            return null;
        running.current = true;
        setBusy(true);
        setMessage("");
        const controller = new AbortController();
        controllers.current.add(controller);
        const timer = setTimeout(() => controller.abort(), 45000);
        try {
            const record = await freezeCustomization(pending.storageKey, i);
            if (!mounted.current)
                return null;
            pending.persist(record);
            const headers = new Headers({ "Content-Type": "application/json", "Idempotency-Key": i.key });
            if (i.version)
                headers.set("If-Match", '"' + i.version + '"');
            const receipt = await customizationRequest(scope, i.path, customReceipt, { method: "POST", headers, body: i.body, signal: controller.signal });
            if (!mounted.current)
                return null;
            pending.clear(record);
            forgetCustomization(pending.storageKey, i.key);
            setMessage("已保存，正在读取最新进度。");
            return receipt;
        }
        catch (e) {
            if (!mounted.current)
                return null;
            setMessage(errorText(e));
            if (e instanceof CustomizationError && ((e.code === "CUSTOMIZATION_INVALID" && e.status === 400) || (e.code === "CUSTOMIZATION_REVISION_MISMATCH" && e.status === 412) || (e.code === "CUSTOMIZATION_NOT_FOUND" && e.status === 404))) {
                const saved = pending.read();
                if (saved) {
                    pending.clear(saved);
                    forgetCustomization(pending.storageKey, i.key);
                }
            }
            return null;
        }
        finally {
            clearTimeout(timer);
            controllers.current.delete(controller);
            running.current = false;
            if (mounted.current)
                setBusy(false);
        }
    }
    const notice = pending.error ? "无法读取原操作记录，已暂停新的提交。请保留浏览器数据并联系平台核实。" : pending.command && !intent ? "原操作尚未确认，附件载荷已无法恢复。已暂停新的提交，请在定制进度中查看并联系平台核实。" : message || pending.command ? message || "原操作尚未确认，请核实同一次操作。" : "";
    return { intent, busy, locked, message: notice, setMessage, execute };
}
type Commands = ReturnType<typeof useCommands>;
function Notice({ commands, retry }: {
    commands: Commands;
    retry: () => void;
}) {
    return commands.message ? <Card className={styles.notice} role="status"><p>{commands.message}</p>{commands.intent && !commands.busy ? <Button variant="outline" onClick={retry}>核实同一次操作</Button> : null}</Card> : null;
}
function ScopedPage({ mode, scope, organizationName }: {
    mode: Mode;
    scope: CustomScope;
    organizationName: string;
}) {
    const title = { intro: "智能体定制", new: "提交定制需求", progress: "定制进度", admin: "智能体定制工作台" }[mode];
    return <ConsolePage className={styles.page} title={title} description={scope.admin ? "硕米平台专员跟进需求评估、方案报价与开发交付。平台权限由服务端独立核实。" : `为业务场景定制智能体能力 · 当前企业：${organizationName}`} breadcrumbs={mode === "intro" ? [{ label: "智能市场" }, { label: title }] : [{ label: "智能市场" }, { label: "智能体定制", href: base }, { label: title }]} actions={mode === "new" ? <Link href={base + "/progress"}>查看定制进度 →</Link> : mode !== "admin" ? <div className={styles.actions}><Button asChild variant="outline"><Link href={base + "/progress"}>查看定制进度</Link></Button><Button asChild><Link href={base + "/new"}>提交定制需求</Link></Button></div> : null}>
  {mode === "intro" ? <Introduction /> : mode === "new" ? <RequestForm scope={scope}/> : <Progress scope={scope}/>}
 </ConsolePage>;
}
function Introduction() {
    return <>
 <Card className={styles.hero}><div><small>围绕你的业务，打造专属智能体</small><h2>让智能体更懂你的业务</h2><p>将重复工作、专业经验和业务规则转化为可使用的智能体能力。先提交需求，由平台专员联系评估。</p></div><aside><strong>有偿定制服务</strong><p>提交需求不收费<br />方案与费用评估后线下确认</p><Button asChild><Link href={base + "/new"}>开始定制</Link></Button></aside></Card>
 <div className={styles.directionGrid}>{directions.map(([id, label], i) => <Card key={id}><span className={styles.number}>0{i + 1}</span><h2>{label}</h2><p>{["商品资料整理、供应链分析与选品辅助", "店铺运营、内容优化与日常工作辅助", "经营数据整理、分析与报告辅助", "围绕你的特定流程与业务需求评估"][i]}</p></Card>)}</div>
 <Card className={styles.process}><h2>定制流程</h2><p>需求评估后确认方案，开发进度由平台专员持续记录。</p><ol>{customStages.map((s, i) => <li key={s}><span className={styles.number}>{i + 1}</span><h3>{stageLabels[s]}</h3><p>{stepDescriptions[i]}</p></li>)}</ol></Card>
 </>;
}
function RequestForm({ scope }: {
    scope: CustomScope;
}) {
    const commands = useCommands(scope), [files, setFiles] = useState<File[]>([]), [fileError, setFileError] = useState(""), [preparing, setPreparing] = useState(false), preparingRef = useRef(false), active = useRef(true), [saved, setSaved] = useState("");
    useEffect(() => { active.current = true; return () => { active.current = false; }; }, []);
    async function send(i: Intent) { const receipt = await commands.execute(i); if (receipt)
        setSaved(receipt.requestId); }
    async function submit(event: React.FormEvent<HTMLFormElement>) {
        event.preventDefault();
        if (commands.locked || preparingRef.current)
            return;
        const values = new FormData(event.currentTarget);
        if (values.get("consent") !== "on") {
            commands.setMessage("请明确同意平台专员联系评估后再提交。");
            return;
        }
        const text = (name: string) => String(values.get(name) ?? "").trim();
        const fields = { name: text("name"), scenario: text("scenario"), direction: text("direction"), description: text("description"), contactName: text("contactName"), contactMethod: text("contactMethod"), consent: true };
        preparingRef.current = true;
        setPreparing(true);
        try {
            const uploads = await Promise.all(files.map(file => new Promise<{
                name: string;
                data: string;
            }>((resolve, reject) => { const reader = new FileReader(); reader.onerror = () => reject(new Error("file")); reader.onload = () => resolve({ name: file.name, data: String(reader.result).split(",")[1] ?? "" }); reader.readAsDataURL(file); })));
            if (!active.current)
                return;
            const parsed = customInput.safeParse({ ...fields, ...(uploads.length ? { files: uploads } : {}) });
            if (!parsed.success) {
                commands.setMessage("请检查必填项、文字长度和附件格式。");
                return;
            }
            await send({ key: crypto.randomUUID(), body: JSON.stringify(parsed.data), path: "" });
        }
        catch {
            if (active.current)
                commands.setMessage("无法读取附件，请重新选择后提交。");
        }
        finally {
            preparingRef.current = false;
            if (active.current)
                setPreparing(false);
        }
    }
    if (saved)
        return <Card className={styles.success} role="status"><h2>定制需求已保存</h2><p>需求编号：{saved}</p><p>平台专员将根据提交的联系方式沟通评估。你可以在当前企业的定制进度中查看需求。</p><Button asChild><Link href={base + "/progress"}>查看已保存需求</Link></Button></Card>;
    return <><Notice commands={commands} retry={() => { if (commands.intent)
        void send(commands.intent); }}/><div className={styles.formGrid}>
  <Card className={styles.formCard}><h2>填写定制需求</h2><p>请尽量描述业务场景、目标和当前操作方式，便于专员评估。</p><form onSubmit={e => void submit(e)}><fieldset disabled={commands.locked || preparing}>
   <div className={styles.twoColumns}><label>需求名称 *<Input name="name" required maxLength={120} placeholder="例如：商品标题优化智能体"/></label><label>主要使用场景 *<Input name="scenario" required maxLength={240} placeholder="例如：商品维护、运营分析"/></label></div>
   <fieldset className={styles.directions}><legend>定制方向 *</legend>{directions.map(([id, label]) => <label key={id}><input type="radio" name="direction" value={id} required/>{label}</label>)}</fieldset>
   <label>希望智能体完成什么？*<textarea name="description" required maxLength={10000} rows={6} placeholder="描述业务背景、输入内容、希望的处理步骤与输出结果。"/></label>
   <label>参考资料（选填）<Input type="file" multiple accept=".pdf,.png,.jpg,.jpeg,.txt,.csv" onChange={e => { const selected = Array.from(e.target.files ?? []); const invalid = selected.length > 3 || selected.some(f => !f.size || f.size > 2 * 1024 * 1024); setFiles(invalid ? [] : selected); setFileError(invalid ? "最多3个附件，每个附件须在2 MiB以内。" : ""); if (invalid)
        e.target.value = ""; }}/></label>
   <p>支持 PDF、PNG、JPEG、纯文本与 CSV；最多3个，每个2 MiB。仅当前企业与有权限的平台专员可下载。</p>{files.map(f => <p key={f.name + f.size}>{f.name} · {Math.ceil(f.size / 1024)} KiB</p>)}{fileError ? <p role="alert">{fileError}</p> : null}
   <div className={styles.twoColumns}><label>联系人 *<Input name="contactName" required maxLength={80} placeholder="怎么称呼你"/></label><label>手机或微信 *<Input name="contactMethod" required maxLength={160} placeholder="用于沟通需求"/></label></div>
   <label className={styles.consent}><input type="checkbox" name="consent" required/>我同意平台专员根据上述联系方式联系我，沟通本次定制需求。</label><p>需求、联系方式和参考资料将对当前企业有权限的成员可见，请确认可以在企业内共享。</p>
   <div className={styles.actions}><Button asChild variant="outline"><Link href={base}>取消</Link></Button><Button type="submit" disabled={!!fileError}>{preparing ? "正在读取资料…" : commands.busy ? "正在提交…" : "提交定制需求"}</Button></div>
  </fieldset></form></Card>
  <aside className={styles.sideCards}><Card><small>有偿定制服务</small><h2>先评估，再确认方案与报价</h2><p>提交定制需求不收费。平台专员联系评估后，与你在线下确认方案、费用和交付安排。</p></Card><Card><h2>提交后会发生什么？</h2><ol>{stepDescriptions.slice(0, 3).map((v, i) => <li key={v}><span>{i + 1}</span>{v}</li>)}</ol></Card><Card><h2>需求填写建议</h2><p>描述当前如何操作、哪些步骤最耗时，并给出希望看到的具体结果。参考资料可帮助专员更准确评估。</p></Card></aside>
 </div></>;
}
function Progress({ scope }: {
    scope: CustomScope;
}) {
    const [cursor, setCursor] = useState(""), [selected, setSelected] = useState(""), [reload, setReload] = useState(0), [result, setResult] = useState<{
        key: string;
        data?: ReturnType<typeof customPage.parse>;
        error?: unknown;
    } | null>(null);
    const { userId, organizationId, admin } = scope, key = cursor + ":" + reload, data = result?.key === key ? result.data : undefined, failure = result?.key === key ? result.error : undefined;
    useEffect(() => { const controller = new AbortController(); const timer = setTimeout(() => controller.abort(), 45000); let alive = true; void customizationRequest({ userId, organizationId, admin }, cursor ? "?cursor=" + cursor : "", customPage, { signal: controller.signal }).then(v => { if (alive)
        setResult({ key, data: v }); }).catch(e => { if (alive)
        setResult({ key, error: e }); }); return () => { alive = false; clearTimeout(timer); controller.abort(); }; }, [userId, organizationId, admin, cursor, key]);
    if (selected)
        return <RequestDetail key={selected} scope={scope} id={selected} close={() => { setSelected(""); setReload(v => v + 1); }}/>;
    return <>{scope.admin ? <p>方案、报价与费用在线下确认；后台记录沟通事实与交付进度。开发记录不代表线上支付或用户验收。</p> : <p>展示当前企业已保存的需求与平台专员记录的进度。</p>}
  {failure ? <Failure error={failure} retry={() => setReload(v => v + 1)}/> : !data ? <ConsoleState kind="loading" title="正在读取定制需求"/> : !data.items.length ? <ConsoleState kind="empty" title={scope.admin ? "暂无待处理的定制需求" : "当前企业暂无定制需求"}/> : <Card className={styles.list}><div className={styles.tableWrap}><table><thead><tr><th>需求</th>{scope.admin ? <th>企业</th> : null}<th>进度</th><th>更新时间</th><th>操作</th></tr></thead><tbody>{data.items.map(r => <tr key={r.id}><td><strong>{r.input.name}</strong><small>{r.input.scenario}</small></td>{scope.admin ? <td>{r.organizationId}</td> : null}<td><span className={styles.stage}>{stageLabels[r.stage]}</span></td><td>{date(r.updatedAt)}</td><td><Button variant="outline" onClick={() => setSelected(r.id)}>查看需求</Button></td></tr>)}</tbody></table></div><div className={styles.actions}><Button variant="outline" onClick={() => { setCursor(""); setReload(v => v + 1); }}>刷新首页</Button>{data.nextCursor ? <Button variant="outline" onClick={() => setCursor(data.nextCursor)}>下一页</Button> : null}</div></Card>}
 </>;
}
function Failure({ error, retry }: {
    error: unknown;
    retry: () => void;
}) { return <ConsoleState kind="error" title={errorText(error)}><Button variant="outline" onClick={retry}>重新读取</Button></ConsoleState>; }
function RequestDetail({ scope, id, close }: {
    scope: CustomScope;
    id: string;
    close: () => void;
}) {
    const commands = useCommands(scope), [result, setResult] = useState<{
        key: string;
        data?: CustomDetail;
        error?: unknown;
    } | null>(null), [reload, setReload] = useState(0), [after, setAfter] = useState(""), [downloadError, setDownloadError] = useState("");
    const { userId, organizationId, admin } = scope, key = after + ":" + reload, data = result?.key === key ? result.data : undefined, failure = result?.key === key ? result.error : undefined;
    const downloads = useRef(new Set<AbortController>()), active = useRef(true);
    useEffect(() => { active.current = true; const running = downloads.current; return () => { active.current = false; running.forEach(c => c.abort()); }; }, []);
    useEffect(() => { const controller = new AbortController(), timer = setTimeout(() => controller.abort(), 45000); let alive = true; void customizationRequest({ userId, organizationId, admin }, "/" + id + (after ? "?after=" + after : ""), customDetail, { signal: controller.signal }).then(v => { if (alive)
        setResult({ key, data: v }); }).catch(e => { if (alive)
        setResult({ key, error: e }); }); return () => { alive = false; clearTimeout(timer); controller.abort(); }; }, [userId, organizationId, admin, id, after, key]);
    async function send(i: Intent) { if (await commands.execute(i)) {
        setAfter("");
        setReload(v => v + 1);
    } }
    async function download(file: NonNullable<CustomDetail>["request"]["attachments"][number]) {
        const controller = new AbortController(), timer = setTimeout(() => controller.abort(), 45000);
        downloads.current.add(controller);
        setDownloadError("");
        try {
            const response = await fetch(`/api/workbench/${scope.admin ? "admin/" : ""}agent-customization/requests/${id}/files/${file.id}`, { headers: { "X-Expected-User-ID": scope.userId, "X-Expected-Organization-ID": scope.organizationId }, credentials: "same-origin", cache: "no-store", redirect: "error", signal: controller.signal });
            if (!response.ok)
                throw new Error();
            const blob = await response.blob();
            if (!active.current)
                return;
            if (blob.size !== file.size || blob.size > 2 * 1024 * 1024)
                throw new Error();
            const url = URL.createObjectURL(blob), a = document.createElement("a");
            a.href = url;
            a.download = file.name;
            a.click();
            setTimeout(() => URL.revokeObjectURL(url), 1000);
        }
        catch {
            if (active.current)
                setDownloadError("无法下载附件，请重新确认权限后重试。");
        }
        finally {
            clearTimeout(timer);
            downloads.current.delete(controller);
        }
    }
    function update(e: React.FormEvent<HTMLFormElement>) { e.preventDefault(); if (!data || commands.locked)
        return; const values = new FormData(e.currentTarget), value = { stage: values.get("stage"), note: String(values.get("note") ?? "").trim(), ...(String(values.get("proposal") ?? "").trim() ? { proposal: String(values.get("proposal")).trim() } : {}), ...(String(values.get("offlineConfirmation") ?? "").trim() ? { offlineConfirmation: String(values.get("offlineConfirmation")).trim() } : {}), ...(values.get("deliverQualityAgent")==="on"?{deliverQualityAgent:true}:{}) }; const parsed = customUpdate.safeParse(value); if (!parsed.success) {
        commands.setMessage("请填写本次跟进说明和必要的方案确认信息。");
        return;
    } void send({ path: "/" + id + "/progress", key: crypto.randomUUID(), body: JSON.stringify(parsed.data), version: data.request.version }); }
    const r = data?.request;
    return <div className={styles.detail}><Button variant="outline" disabled={commands.locked} onClick={close}>返回需求列表</Button><Notice commands={commands} retry={() => { if (commands.intent)
        void send(commands.intent); }}/>{failure ? <Failure error={failure} retry={() => setReload(v => v + 1)}/> : !r ? <ConsoleState kind="loading" title="正在读取需求与进度"/> : <>
  <Card><div className={styles.detailHeading}><h2>{r.input.name}</h2><span className={styles.stage}>{stageLabels[r.stage]}</span></div><dl><dt>需求编号</dt><dd>{r.id}</dd><dt>当前企业</dt><dd>{r.organizationId}</dd><dt>使用场景</dt><dd>{r.input.scenario}</dd><dt>定制方向</dt><dd>{directions.find(d => d[0] === r.input.direction)?.[1]}</dd><dt>具体需求</dt><dd className={styles.multiline}>{r.input.description}</dd><dt>联系人</dt><dd>{r.input.contactName} · {r.input.contactMethod}</dd><dt>联系授权</dt><dd>提交时已明确同意平台专员联系评估</dd><dt>提交时间</dt><dd>{date(r.createdAt)}</dd><dt>方案与报价</dt><dd className={styles.multiline}>{r.proposal || "尚未记录"}</dd><dt>线下确认记录</dt><dd className={styles.multiline}>{r.offlineConfirmation || "尚未记录"}</dd></dl>{r.attachments.length ? <div className={styles.actions}>{r.attachments.map(f => <Button key={f.id} variant="outline" onClick={() => void download(f)}>下载 {f.name}</Button>)}</div> : <p>未附参考资料</p>}{downloadError ? <p role="alert">{downloadError}</p> : null}</Card>
  <Card><h2>跟进记录</h2><ol className={styles.events}>{data!.events.map(event => <li key={event.version}><div><strong>{stageLabels[event.stage]}</strong><time>{date(event.at)}</time></div><p className={styles.multiline}>{event.note}</p>{event.proposal ? <p className={styles.multiline}>方案与报价：{event.proposal}</p> : null}{event.offlineConfirmation ? <p className={styles.multiline}>线下确认：{event.offlineConfirmation}</p> : null}</li>)}</ol><div className={styles.actions}>{after ? <Button variant="outline" disabled={commands.locked} onClick={() => setAfter("")}>最早记录</Button> : null}{data!.nextEventVersion ? <Button variant="outline" disabled={commands.locked} onClick={() => setAfter(data!.nextEventVersion)}>后续记录</Button> : null}<Button variant="outline" onClick={() => setReload(v => v + 1)}>刷新进度</Button></div></Card>
  <Card><h2>实际智能体交付</h2>{r.deliveryId ? <><p>仅需求所属企业可使用；具体业务与版本以智能体详情为准。</p>{scope.admin ? <p>客户可从所属企业的“我的智能体”进入使用。</p> : <Button asChild><Link href={"/workbench/agents/mine/private/"+r.deliveryId}>进入私有智能体</Link></Button>}</> : <p>尚未发布可执行智能体；人工交付记录仅说明处理进度。</p>}</Card>
  {scope.admin ? <Card><h2>记录本次跟进</h2><p>只能保留当前阶段或进入下一阶段。方案与费用线下确认后，才能开始开发。</p><StaffForm key={r.version} request={r} commands={commands} update={update}/></Card> : null}
 </>}</div>;
}
function StaffForm({ request: r, commands, update }: {
    request: CustomRequest;
    commands: Commands;
    update: (e: React.FormEvent<HTMLFormElement>) => void;
}) {
    const [stage, setStage] = useState(r.stage), needsConfirmation = r.stage === "PROPOSED" && stage === "DEVELOPING";
    return <form onSubmit={update}><fieldset disabled={commands.locked}><label>本次阶段<select name="stage" value={stage} onChange={e => setStage(e.target.value as CustomRequest["stage"])}>{customStages.slice(customStages.indexOf(r.stage), customStages.indexOf(r.stage) + 2).map(s => <option key={s} value={s}>{stageLabels[s]}</option>)}</select></label><label>跟进说明 *<textarea name="note" required maxLength={5000} rows={3}/></label>{stage === "PROPOSED" ? <label>方案与报价（进入方案确认时必填）<textarea name="proposal" required maxLength={5000} rows={3} defaultValue={r.proposal}/></label> : null}{needsConfirmation ? <label>线下确认记录（开始开发前必填）<textarea name="offlineConfirmation" required maxLength={5000} rows={3}/></label> : null}{stage==="DELIVERED"&&!r.deliveryId?<><label><input type="checkbox" name="deliverQualityAgent"/>交付平台草稿资料质检智能体 v2.0.0</label><p>仅授权给本需求企业 {r.organizationId}。选择上传前的已保存平台草稿，复用保存时校验；不调用模型，不修改商品。</p></>:null}<p>请记录线下确认时间、对象与约定。此记录不会确认付款，也不会代替用户验收。</p><Button type="submit">{commands.busy ? "正在保存…" : "保存跟进记录"}</Button></fieldset></form>;
}
