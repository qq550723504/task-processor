"use client";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { requestProductAgent, ProductAgentError } from "@/lib/api/product-agent";
import { isAcquisitionUUID } from "@/lib/contracts/product-acquisition";
import type { ProductAgentResult } from "@/lib/contracts/product-agent";
import { agentStartRequestSchema } from "@/lib/contracts/product-agent";
import type { z } from "zod";
import {TitleAgentTemplates} from "./title-agent-templates";
import type {AgentTemplate} from "@/lib/contracts/agent-configuration";
import { TitleAgentConfirmation } from "./title-agent-confirmation";
import { KnowledgeCitations } from "../task-center/knowledge-citations";
const reasons: Record<string, string> = { budget_steps: "已达到步骤上限", budget_model_calls: "已达到模型调用上限", budget_tokens: "剩余 token 预算不足", budget_cost: "剩余费用预算不足", budget_runtime: "已达到运行时限", usage_unknown: "用量尚未确认，预留额度仍保留", model_outcome_unknown: "模型结果尚未确认，请勿重新生成", unauthorized: "当前权限不可用", dependency_unavailable: "模型配置、计费策略或依赖尚未就绪", invalid_model_output: "模型输出格式无效", repair_limit: "两次修复后仍未通过校验", audit_unavailable: "调用记录保存失败", tool_error: "证据工具读取失败" };
const errors: Record<string, string> = { PRODUCT_AGENT_UNAVAILABLE: "当前企业尚未开放标题诊断，或文本模型配置尚未就绪。", OUTCOME_UNKNOWN: "请求结果尚未确认。请保留本次编号，读取当前结果，不要重新生成。", AGENT_CONFLICT: "运行状态已经变化，请读取当前结果。", CONFIGURATION_CHANGED:"配置准入已变化，请核实当前配置与原运行状态。",AGENT_DEFINITION_UNAVAILABLE:"当前执行引用的智能体版本不可用，请核实当前版本与原运行状态。",AGENT_NOT_ENABLED:"当前企业智能体已停用，请由管理员重新启用。",TEMPLATE_ARCHIVED:"所选模板已归档，请核实当前结果和模板。",IDEMPOTENCY_CONFLICT:"本次编号已绑定其他配置，请保留编号并读取当前结果。", FORBIDDEN: "当前身份没有操作权限。", ORGANIZATION_ACCESS_REVOKED: "当前企业权限已撤销。", DEPENDENCY_UNAVAILABLE: "暂时无法读取结果，请稍后用同一编号查询。" };
const knowledgeStartErrors:Record<string,string>={KNOWLEDGE_CONTEXT_TOO_LARGE:"所选知识完整内容超过输入上限，本次未启动。请缩小知识范围后重新确认。",KNOWLEDGE_NOT_READY:"所选知识没有完整可读版本，本次未启动。",KNOWLEDGE_DISABLED:"所选知识已停用，本次未启动。",KNOWLEDGE_NOT_FOUND:"所选知识当前不可用，本次未启动。",KNOWLEDGE_FORBIDDEN:"当前身份无权读取所选知识，本次未启动。"};
type StartIntent=z.infer<typeof agentStartRequestSchema>;
function restoredStart(params:{get:(name:string)=>string|null},userId:string,organizationId:string):StartIntent|null {
    if(!isAcquisitionUUID(params.get("agent_key")??"")||params.get("agent_actor")!==userId||params.get("agent_org")!==organizationId)return null;
    const baseId=params.get("agent_knowledge_base");
    const templateId=params.get("agent_template"),revision=params.get("agent_template_version");
    const parsed=agentStartRequestSchema.safeParse({...(templateId!==null||revision!==null?{templateSelection:{templateId,revision}}:{}),targetPlatform:params.get("agent_platform"),...(baseId!==null?{knowledgeSelection:{knowledgeBaseId:baseId}}:{})});
    return parsed.success?parsed.data:null;
}
const intentParams=["agent_key","agent_platform","agent_knowledge_base","agent_actor","agent_org","agent_template","agent_template_version"];
export function ProductAgentPanel({ enabled, knowledgeAvailable=false, operationId, productKey, catalogVersion }: {
    enabled: boolean;
    knowledgeAvailable?:boolean;
    operationId: string;
    productKey: string;
    catalogVersion: string;
}) {
    const context = useWorkbenchContext();
    const params = useSearchParams();
    const userId = context.user?.id, organizationId = context.effectiveOrganization?.id;
    if (!enabled)
        return <Card><h2>标题诊断</h2><p>当前环境尚未开放 Product Agent。</p></Card>;
    if (!userId || !organizationId || context.isSwitching || context.isLoading || context.error || context.blockingError || context.selectionRequired)
        return null;
    return <ScopedAgentPanel key={`${userId}:${organizationId}:${operationId}`} knowledgeAvailable={knowledgeAvailable} userId={userId} organizationId={organizationId} operationId={operationId} productKey={productKey} catalogVersion={catalogVersion} initialKey={params.get("agent_key") ?? ""} initialStart={restoredStart(params,userId,organizationId)}/>;
}
function ScopedAgentPanel({ userId, organizationId, operationId, productKey, catalogVersion, initialKey,initialStart,knowledgeAvailable }: {
    knowledgeAvailable:boolean;
    userId: string;
    organizationId: string;
    operationId: string;
    productKey: string;
    catalogVersion: string;
    initialKey: string;
    initialStart:StartIntent|null;
}) {
    const context = useWorkbenchContext();
    const [key, setKey] = useState(isAcquisitionUUID(initialKey) ? initialKey : "");
    // Public normalized command only; protected Knowledge payload and authority
    // stay with the server. Explicit replay never replaces an uncertain key.
    const [intent,setIntent]=useState<StartIntent|null>(initialStart);
    const rolesKey=JSON.stringify([...context.roles].sort());
    const [authority,setAuthority]=useState({rolesKey,generation:0});
    const generation=authority.rolesKey===rolesKey?authority.generation:authority.generation+1;
    if(authority.rolesKey!==rolesKey)setAuthority({rolesKey,generation});
    const knowledgeReadable=context.roles.some(role=>["listingkit_admin","platform_admin","listingkit_operator"].includes(role));
    const [result, setResult] = useState<(ProductAgentResult & {knowledgeGeneration:number}) | null>(null);
    // Invalidate protected display during render, including late responses and
    // downgrade/regrant. Keep the durable operation key and all UNKNOWN intent.
    if(result&&result.knowledgeGeneration!==generation) {
        setResult({...result,knowledgeGeneration:generation,knowledge:result.knowledge?{status:"unavailable",originAgentRunId:result.knowledge.originAgentRunId,citations:[]}:undefined});
    }
    const [proposal, setProposal] = useState("");
    const [feedback, setFeedback] = useState("");
    const [platform, setPlatform] = useState<string>(initialStart?.targetPlatform??"");
    const [busy, setBusy] = useState(false);
    const [failure, setFailure] = useState("");
    const [reconfirmable,setReconfirmable]=useState(false);
    const [confirming,setConfirming]=useState(false);
    const [template,setTemplate]=useState<AgentTemplate|null>(null),[templateReady,setTemplateReady]=useState(false);
    const active = useRef(true), inFlight = useRef(false), abort = useRef<AbortController | null>(null);
    useEffect(() => { active.current = true; return () => { active.current = false; abort.current?.abort(); }; }, []);
    useEffect(() => context.registerOrganizationSwitchGuard(() => !inFlight.current), [context]);
    async function execute(action: "start" | "read" | "resume" | "review",knowledgeBaseId?:string) {
        const start=action==="start"?intent??agentStartRequestSchema.safeParse({targetPlatform:platform,...(template?{templateSelection:{templateId:template.templateId,revision:template.version}}:{}),...(knowledgeBaseId?{knowledgeSelection:{knowledgeBaseId}}:{})}).data:null;
        if (inFlight.current || !active.current || action === "start" && !start)
            return;
        const freshStart=action==="start"&&!key;
        const requestKey = key || crypto.randomUUID();
        if (!key) {
            setKey(requestKey);
            const url = new URL(window.location.href);
            url.searchParams.set("agent_key", requestKey);
            if(start){
                setIntent(start);url.searchParams.set("agent_platform",start.targetPlatform);
                url.searchParams.set("agent_actor",userId);url.searchParams.set("agent_org",organizationId);
                url.searchParams.delete("agent_template");url.searchParams.delete("agent_template_version");
                if(start.templateSelection){url.searchParams.set("agent_template",start.templateSelection.templateId);url.searchParams.set("agent_template_version",start.templateSelection.revision)}
                url.searchParams.delete("agent_knowledge_base");
                if(start.knowledgeSelection)url.searchParams.set("agent_knowledge_base",start.knowledgeSelection.knowledgeBaseId);
            }
            window.history.replaceState(null, "", url);
        }
        const controller = new AbortController();
        abort.current = controller;
        inFlight.current = true;
        setBusy(true);
        setFailure("");
        setReconfirmable(false);
        try {
            const next = await requestProductAgent(action, operationId, requestKey, { userId, organizationId }, controller.signal, result?.revision, feedback, start?.targetPlatform??platform,start?.knowledgeSelection?.knowledgeBaseId,start?.templateSelection);
            if (!active.current)
                return;
            if ("proposalId" in next) {
                setProposal(next.proposalId);
                return;
            }
            if (next.productKey !== productKey || next.catalogVersion !== catalogVersion || platform && next.targetPlatform !== platform)
                throw new ProductAgentError("AGENT_CONFLICT");
            setPlatform(next.targetPlatform);
            setResult({...next,knowledgeGeneration:generation});
        }
        catch (error) {
            if (active.current) {
                setFailure(error instanceof ProductAgentError ? error.code : "OUTCOME_UNKNOWN");
                setReconfirmable(action==="start"&&!result&&error instanceof ProductAgentError&&["CONFIGURATION_CHANGED","TEMPLATE_ARCHIVED","AGENT_NOT_ENABLED","AGENT_DEFINITION_UNAVAILABLE"].includes(error.code));
                if(freshStart&&error instanceof ProductAgentError&&knowledgeStartErrors[error.code]){
                    setKey("");setIntent(null);const url=new URL(window.location.href);for(const name of intentParams)url.searchParams.delete(name);window.history.replaceState(null,"",url);
                }
                if (error instanceof ProductAgentError && (error.code === "FORBIDDEN" || error.code.startsWith("ORGANIZATION_"))) {
                    setResult(null);
                    setProposal("");
                }
            }
        }
        finally {
            inFlight.current = false;
            if (active.current)
                setBusy(false);
        }
    }
    return <Card className="space-y-3 p-5"><h2 className="text-lg font-semibold">标题诊断与补全</h2>
  <p>从当前已保存版本读取证据，生成标题建议。最多修复两次；提交后仍需人工审核，不会自动修改商品。</p>
  {!key ? <><TitleAgentTemplates key={rolesKey} scope={{userId,organizationId}} onReady={setTemplateReady} onChoose={value=>{setTemplate(value);if(value)setPlatform(value.targetPlatform)}}/><label htmlFor="agent-platform">素材查询平台</label><select id="agent-platform" className="block rounded-lg border p-2" value={platform} onChange={event => setPlatform(event.target.value)} disabled={busy}><option value="">请选择平台</option><option value="shein">SHEIN</option><option value="temu">Temu</option><option value="amazon">Amazon</option></select><p className="text-sm">用于读取该平台已批准的素材；本次诊断不评估平台发布规则。</p><Button disabled={busy || !platform || !templateReady} onClick={() => setConfirming(true)}>生成标题建议</Button></> : <><p className="break-all text-sm">本次编号：{key}</p><Button variant="outline" disabled={busy} onClick={() => void execute("read")}>读取当前结果</Button>{intent&&!result&&<><p className="text-sm">原请求平台：{intent.targetPlatform}；{intent.knowledgeSelection?`知识库编号：${intent.knowledgeSelection.knowledgeBaseId}`:"未选择企业知识"}。恢复将沿用同一编号和选择；服务端已有运行时只返回该运行，不会重新生成。</p><Button variant="outline" disabled={busy} onClick={()=>void execute("start")}>使用原请求核实启动</Button></>}</>}
  {confirming&&!key&&<TitleAgentConfirmation key={rolesKey} scope={{userId,organizationId}} organization={context.effectiveOrganization?.name||organizationId} platform={platform} template={template??undefined} knowledgeAvailable={knowledgeAvailable} knowledgeReadable={knowledgeReadable} onCancel={()=>setConfirming(false)} onConfirm={baseId=>{setConfirming(false);void execute("start",baseId)}}/>}
  {busy && <p role="status">正在处理，请保持页面打开。离开页面可能使已发送的调用结果未知；返回后请用本次编号读取，勿重新生成。</p>}
  {failure && <p role="alert">{reconfirmable?`${errors[failure]} 本次未领取执行。请明确重新确认配置，再使用新编号生成。`:key&&knowledgeStartErrors[failure]?"知识当前不可用，已保留原编号；已有运行结果尚需核实，请勿换编号重新生成。":knowledgeStartErrors[failure] ?? errors[failure] ?? "请求未完成，请核对当前身份和企业。"}</p>}
  {reconfirmable&&<Button variant="outline" disabled={busy} onClick={()=>{if(inFlight.current)return;setKey("");setIntent(null);setTemplate(null);setTemplateReady(false);setPlatform("");setFailure("");setReconfirmable(false);const url=new URL(window.location.href);for(const name of intentParams)url.searchParams.delete(name);window.history.replaceState(null,"",url)}}>重新确认配置</Button>}
  {result && <div className="space-y-2">{result.templateSelection&&<p>采用模板版本：{result.templateSelection.revision} · {result.templateSelection.templateId}</p>}<p>素材查询平台：{{ shein: "SHEIN", temu: "Temu", amazon: "Amazon" }[result.targetPlatform]}</p><p>状态：{result.phase === "human_review_required" ? (result.canSubmitReview ? "候选已通过校验，等待人工审核" : "候选未通过校验，不能提交人工审核") : result.phase === "interrupted" ? "需要补充说明" : result.phase === "running" ? "运行记录尚未完成，不能重复发送" : "已停止"}</p>
   {result.stopReason && <p>{reasons[result.stopReason] ?? "本次运行已停止"}</p>}
   {result.candidate.Changes?.map((change, index) => <div key={index}><p>{change.Field}：{change.Value}</p><p className="text-sm">证据：{change.EvidenceIDs?.join("、") || "未提供"}</p></div>)}
   <KnowledgeCitations knowledge={result.knowledge&&(!knowledgeReadable||result.knowledgeGeneration!==generation)?{status:"unavailable",originAgentRunId:result.knowledge.originAgentRunId,citations:[]}:result.knowledge}/>
   {result.confidence?.map(value => <p key={value.Field}>模型自报置信度（供参考）：{value.Field} {value.Known ? `${Math.round(value.Value * 100)}%` : "未知"}</p>)}
   {result.unresolved?.length ? <ul>{result.unresolved.map((item, index) => <li key={index}>{item}</li>)}</ul> : null}
   <p className="text-sm">{result.usageStatus === "observed" ? `已观测 ${result.tokens} tokens，预算估价 ${(result.estimatedCostMicros / 1000000).toFixed(4)} ${result.currency}` : "用量尚未确认，显示的预留不能当作实际账单。"}</p>
   <details><summary>查看调用记录</summary><ol>{result.steps.map(step => <li key={step.step}>{step.step}：{step.tool || "文本模型"} · {step.callId}</li>)}</ol></details>
   {result.phase === "interrupted" && <><label htmlFor="agent-feedback">补充说明</label><Input id="agent-feedback" value={feedback} onChange={e => setFeedback(e.target.value)} disabled={busy}/><Button disabled={busy || !feedback.trim()} onClick={() => void execute("resume")}>继续本次诊断</Button></>}
   {result.canSubmitReview && !proposal && <Button disabled={busy} onClick={() => void execute("review")}>提交人工审核</Button>}
  </div>}
  {proposal && <Button asChild variant="outline"><Link href={`/workbench/ai/tasks/pending?proposal_id=${proposal}`}>打开标题审核</Link></Button>}
 </Card>;
}
