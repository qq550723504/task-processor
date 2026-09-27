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
const reasons: Record<string, string> = { budget_steps: "已达到步骤上限", budget_model_calls: "已达到模型调用上限", budget_tokens: "剩余 token 预算不足", budget_cost: "剩余费用预算不足", budget_runtime: "已达到运行时限", usage_unknown: "用量尚未确认，预留额度仍保留", model_outcome_unknown: "模型结果尚未确认，请勿重新生成", unauthorized: "当前权限不可用", dependency_unavailable: "模型配置、计费策略或依赖尚未就绪", invalid_model_output: "模型输出格式无效", repair_limit: "两次修复后仍未通过校验", audit_unavailable: "调用记录保存失败", tool_error: "证据工具读取失败" };
const errors: Record<string, string> = { PRODUCT_AGENT_UNAVAILABLE: "当前企业尚未开放标题诊断，或文本模型配置尚未就绪。", OUTCOME_UNKNOWN: "请求结果尚未确认。请保留本次编号，读取当前结果，不要重新生成。", AGENT_CONFLICT: "运行状态已经变化，请读取当前结果。", FORBIDDEN: "当前身份没有操作权限。", ORGANIZATION_ACCESS_REVOKED: "当前企业权限已撤销。", DEPENDENCY_UNAVAILABLE: "暂时无法读取结果，请稍后用同一编号查询。" };
export function ProductAgentPanel({ enabled, operationId, productKey, catalogVersion }: {
    enabled: boolean;
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
    return <ScopedAgentPanel key={`${userId}:${organizationId}:${operationId}`} userId={userId} organizationId={organizationId} operationId={operationId} productKey={productKey} catalogVersion={catalogVersion} initialKey={params.get("agent_key") ?? ""}/>;
}
function ScopedAgentPanel({ userId, organizationId, operationId, productKey, catalogVersion, initialKey }: {
    userId: string;
    organizationId: string;
    operationId: string;
    productKey: string;
    catalogVersion: string;
    initialKey: string;
}) {
    const context = useWorkbenchContext();
    const [key, setKey] = useState(isAcquisitionUUID(initialKey) ? initialKey : "");
    const [result, setResult] = useState<ProductAgentResult | null>(null);
    const [proposal, setProposal] = useState("");
    const [feedback, setFeedback] = useState("");
    const [platform, setPlatform] = useState("");
    const [busy, setBusy] = useState(false);
    const [failure, setFailure] = useState("");
    const active = useRef(true), inFlight = useRef(false), abort = useRef<AbortController | null>(null);
    useEffect(() => { active.current = true; return () => { active.current = false; abort.current?.abort(); }; }, []);
    useEffect(() => context.registerOrganizationSwitchGuard(() => !inFlight.current), [context]);
    async function execute(action: "start" | "read" | "resume" | "review") {
        if (inFlight.current || !active.current || action === "start" && !platform)
            return;
        const requestKey = key || crypto.randomUUID();
        if (!key) {
            setKey(requestKey);
            const url = new URL(window.location.href);
            url.searchParams.set("agent_key", requestKey);
            window.history.replaceState(null, "", url);
        }
        const controller = new AbortController();
        abort.current = controller;
        inFlight.current = true;
        setBusy(true);
        setFailure("");
        try {
            const next = await requestProductAgent(action, operationId, requestKey, { userId, organizationId }, controller.signal, result?.revision, feedback, platform);
            if (!active.current)
                return;
            if ("proposalId" in next) {
                setProposal(next.proposalId);
                return;
            }
            if (next.productKey !== productKey || next.catalogVersion !== catalogVersion || platform && next.targetPlatform !== platform)
                throw new ProductAgentError("AGENT_CONFLICT");
            setPlatform(next.targetPlatform);
            setResult(next);
        }
        catch (error) {
            if (active.current) {
                setFailure(error instanceof ProductAgentError ? error.code : "OUTCOME_UNKNOWN");
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
  {!key ? <><label htmlFor="agent-platform">素材查询平台</label><select id="agent-platform" className="block rounded-lg border p-2" value={platform} onChange={event => setPlatform(event.target.value)} disabled={busy}><option value="">请选择平台</option><option value="shein">SHEIN</option><option value="temu">Temu</option><option value="amazon">Amazon</option></select><p className="text-sm">用于读取该平台已批准的素材；本次诊断不评估平台发布规则。</p><Button disabled={busy || !platform} onClick={() => void execute("start")}>生成标题建议</Button></> : <><p className="break-all text-sm">本次编号：{key}</p><Button variant="outline" disabled={busy} onClick={() => void execute("read")}>读取当前结果</Button></>}
  {busy && <p role="status">正在处理，请保留当前页面。停止等待不会撤销已经发送的模型请求。</p>}
  {failure && <p role="alert">{errors[failure] ?? "请求未完成，请核对当前身份和企业。"}</p>}
  {result && <div className="space-y-2"><p>素材查询平台：{{ shein: "SHEIN", temu: "Temu", amazon: "Amazon" }[result.targetPlatform]}</p><p>状态：{result.phase === "human_review_required" ? (result.canSubmitReview ? "候选已通过校验，等待人工审核" : "候选未通过校验，不能提交人工审核") : result.phase === "interrupted" ? "需要补充说明" : result.phase === "running" ? "运行记录尚未完成，不能重复发送" : "已停止"}</p>
   {result.stopReason && <p>{reasons[result.stopReason] ?? "本次运行已停止"}</p>}
   {result.candidate.Changes?.map((change, index) => <div key={index}><p>{change.Field}：{change.Value}</p><p className="text-sm">证据：{change.EvidenceIDs?.join("、") || "未提供"}</p></div>)}
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
