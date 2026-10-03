"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ConsolePage, ConsoleState } from "../console/console-page";
import { requestAIWorkbench, type AIScope, AIWorkbenchError } from "@/lib/api/ai-workbench";
import { isAcquisitionUUID } from "@/lib/contracts/product-acquisition";
import { aiMessageBody, type AIConversation, type AIProposal } from "@/lib/contracts/ai-workbench";
import { findConsoleRoute } from "@/lib/workbench/console-navigation";
import styles from "./chat-page.module.css";

type ChatMode = "home" | "new" | "recent" | "saved" | "archived";
type MessageBody = ReturnType<typeof aiMessageBody.parse>;
type PendingMessage = { key: string; body: MessageBody };
type ConfirmationReceipt = { key: string; taskId?: string };
const pendingMessageStorageKey = (scope: AIScope, id: string) => `ai-workbench:pending-message:${scope.userId}:${scope.organizationId}:${id}`;
const pendingCreateStorageKey = (scope: AIScope) => `ai-workbench:pending-create:${scope.userId}:${scope.organizationId}`;
const confirmStorageKey = (scope: AIScope, id: string) => `ai-workbench:confirm:${scope.userId}:${scope.organizationId}:${id}`;
const stateText: Record<string, string> = {
  OUTCOME_UNKNOWN: "结果暂无法确认。可用相同操作键重试，系统会读取原收据。",
  PROPOSAL_STALE: "商品或会话已变化，请刷新并重新提出方案。",
  REVISION_MISMATCH: "会话内容已变化，请刷新后重试当前编辑。",
  DEPENDENCY_UNAVAILABLE: "服务暂不可用，请稍后重试。",
  FORBIDDEN: "当前企业或权限已变化，请重新加载。",
  CONVERSATION_ARCHIVED: "该会话已归档，不能继续发送消息。",
};
function errorText(error: unknown) { return error instanceof AIWorkbenchError ? stateText[error.code] ?? `请求未完成：${error.code}` : "请求暂不可用，请稍后重试。"; }

export function ChatPage({ mode, conversationId }: { mode: ChatMode | "detail"; conversationId?: string }) {
  const context = useWorkbenchContext();
  const route = findConsoleRoute(mode === "detail" ? "/workbench/ai/chat" : mode === "home" ? "/workbench/ai/chat" : `/workbench/ai/chat/${mode}`);
  const scope = context.user && context.effectiveOrganization && !context.selectionRequired && !context.isSwitching && !context.error && !context.blockingError
    ? { userId: context.user.id, organizationId: context.effectiveOrganization.id } : null;
  const canUse = context.effectiveOrganization?.capabilities?.["workbench.chat.use"] === true;
  const planningReadiness = context.aiWorkbenchPlanningReadiness;
  const titleReadiness = context.aiWorkbenchTitleReadiness;
  const canPlan = canUse && planningReadiness === "AVAILABLE";
  const authorizationKey = `${context.roles.join(",")}:${canUse}:${planningReadiness}:${titleReadiness}`;
  const title = mode === "home" ? "硕米Chat" : mode === "new" ? "新建会话" : mode === "recent" ? "最近会话" : mode === "saved" ? "收藏会话" : mode === "archived" ? "归档会话" : "业务会话";
  return <ConsolePage className="console-chat" title={title} breadcrumbs={route?.trail}
    description="围绕已保存商品讨论标题建议；方案需要确认后才会执行，结果仍须人工审核应用。">
    {!scope ? <ConsoleState kind={context.isSwitching || context.isLoading ? "loading" : "unavailable"} title="企业上下文不可用">请先选择可访问的企业并登录。</ConsoleState> :
      !context.aiWorkbenchAvailable ? <ConsoleState kind="unavailable" title="暂未启用">当前应用未启用硕米 Chat 与业务任务。</ConsoleState> :
      <ScopedChat key={`${scope.userId}:${scope.organizationId}:${authorizationKey}`} scope={scope} authorizationKey={authorizationKey} canUse={canUse} canPlan={canPlan} planningReadiness={planningReadiness} titleReadiness={titleReadiness} mode={mode} conversationId={conversationId} />}
  </ConsolePage>;
}

function ScopedChat({ scope, authorizationKey, canUse, canPlan, planningReadiness, titleReadiness, mode, conversationId }: { scope: AIScope; authorizationKey: string; canUse: boolean; canPlan: boolean; planningReadiness: "AVAILABLE" | "NEEDS_CONFIGURATION" | "UNAVAILABLE"; titleReadiness: "AVAILABLE" | "NEEDS_CONFIGURATION" | "UNAVAILABLE"; mode: ChatMode | "detail"; conversationId?: string }) {
  if (mode === "detail" && conversationId) return <ConversationDetail scope={scope} authorizationKey={authorizationKey} canUse={canUse} canPlan={canPlan} planningReadiness={planningReadiness} titleReadiness={titleReadiness} id={conversationId} />;
  return <ChatCollection scope={scope} authorizationKey={authorizationKey} canUse={canUse} canPlan={canPlan} planningReadiness={planningReadiness} mode={mode === "detail" ? "home" : mode} />;
}

function planningStatus(readiness: "AVAILABLE" | "NEEDS_CONFIGURATION" | "UNAVAILABLE") {
  return readiness === "NEEDS_CONFIGURATION" ? "当前企业的规划模型需要配置；已有会话仍可查看。" : "当前企业的标题规划尚未开放；已有会话仍可查看。";
}

function ChatCollection({ scope, authorizationKey, canUse, canPlan, planningReadiness, mode }: { scope: AIScope; authorizationKey: string; canUse: boolean; canPlan: boolean; planningReadiness: "AVAILABLE" | "NEEDS_CONFIGURATION" | "UNAVAILABLE"; mode: ChatMode }) {
  const router = useRouter();
  const search = useSearchParams();
  const selectedOperationId = search.get("operationId");
  const [createKey, setCreateKey] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");
  const abort = useRef<AbortController | null>(null);
  const createStorageKey = pendingCreateStorageKey(scope);
  useEffect(() => {
    let active = true;
    queueMicrotask(() => { if (active) { try { setCreateKey(sessionStorage.getItem(createStorageKey) ?? ""); } catch { /* storage may be unavailable */ } } });
    return () => { active = false; };
  }, [createStorageKey]);
  useEffect(() => () => abort.current?.abort(), []);
  const conversations = useInfiniteQuery({ queryKey: ["ai-chat", scope.userId, scope.organizationId, authorizationKey, "list", mode], initialPageParam: "",
    queryFn: ({ signal, pageParam }) => requestAIWorkbench({ route: "conversation-list", method: "GET", path: `chat/conversations?limit=50${mode === "saved" ? "&saved=true" : mode === "archived" ? "&archived=true" : ""}${pageParam ? `&after=${pageParam}` : ""}`, scope, signal }),
    getNextPageParam: page => page.next || undefined,
    retry: false, staleTime: 0, refetchOnWindowFocus: false });
  const visible = useMemo(() => {
    const seen = new Set<string>();
    return (conversations.data?.pages.flatMap(page => page.conversations) ?? []).filter(item => {
      if (seen.has(item.ID) || item.Archived !== (mode === "archived") || (mode === "saved" && !item.Favorite)) return false;
      seen.add(item.ID);
      return true;
    });
  }, [conversations.data, mode]);
  async function create() {
    const key = createKey || crypto.randomUUID();
    setCreateKey(key); setCreating(true); setError("");
    try { sessionStorage.setItem(pendingCreateStorageKey(scope), key); } catch { /* retain the in-memory key */ }
    const controller = new AbortController(); abort.current = controller;
    try {
      const result = await requestAIWorkbench({ route: "conversation-create", method: "POST", path: "chat/conversations", scope, key, body: {}, signal: controller.signal });
      if (controller.signal.aborted) return;
      setCreateKey("");
      try { sessionStorage.removeItem(pendingCreateStorageKey(scope)); } catch { /* navigation still succeeds */ }
      router.push(`/workbench/ai/chat/${result.conversation.ID}${selectedOperationId && isAcquisitionUUID(selectedOperationId) ? `?operationId=${selectedOperationId}` : ""}`);
    } catch (cause) { if (!controller.signal.aborted) setError(errorText(cause)); }
    finally { if (!controller.signal.aborted) setCreating(false); abort.current = null; }
  }
  return <>
    {canUse && !canPlan ? <ConsoleState kind="unavailable" title="规划暂不可用">{planningStatus(planningReadiness)}</ConsoleState> : null}
    {mode === "home" ? <>
      <Card className={styles.summary}>当前企业：{scope.organizationId} · 实际会话按当前账号读取。执行标题建议会消耗已配置的 AI 额度。</Card>
      <div className={styles.landingGrid}>
        <Card className={`${styles.landingCard} ${styles.newCard}`}><span className={styles.accent} /><h2>新建会话</h2><h3>开始一项新需求</h3><p>选择已有商品采集结果，描述标题优化目标，再查看并确认方案。</p>{canPlan ? <Button onClick={create} disabled={creating}>{creating ? "正在创建…" : "进入新建会话 →"}</Button> : <p>{canUse ? "当前企业的规划模型尚未就绪。" : "当前企业仅可查看已有会话。"}</p>}</Card>
        <Card className={`${styles.landingCard} ${styles.recentCard}`}><span className={styles.accent} /><h2>最近会话</h2><h3>继续之前的工作</h3><p>查看当前账号的对话与提案，从上次讨论的位置继续。</p><Button asChild variant="outline"><Link href="/workbench/ai/chat/recent" prefetch={false}>查看最近会话 →</Link></Button></Card>
        <Card className={`${styles.landingCard} ${styles.savedCard}`}><span className={styles.accent} /><h2>收藏会话</h2><h3>沉淀重要内容</h3><p>收藏仍在使用的会话，随时继续查看已保存的内容。</p><Button asChild variant="outline"><Link href="/workbench/ai/chat/saved" prefetch={false}>进入收藏会话 →</Link></Button></Card>
      </div>
    </> : <div className={styles.toolbar}>{canPlan ? <Button onClick={create} disabled={creating}>{creating ? "正在创建…" : "新建会话"}</Button> : null}
      {mode === "archived" ? <Button asChild variant="outline"><Link href="/workbench/ai/chat/recent" prefetch={false}>返回最近会话</Link></Button> :
        mode === "recent" ? <Button asChild variant="outline"><Link href="/workbench/ai/chat/archived" prefetch={false}>查看归档会话</Link></Button> : null}
      <Button variant="outline" onClick={() => void conversations.refetch()}>刷新</Button></div>}
    {error ? <ConsoleState kind="error" title="操作未完成">{error}</ConsoleState> : null}
    {conversations.isPending ? <ConsoleState kind="loading" title="正在读取会话" /> : conversations.isError && !conversations.data ? <ConsoleState kind="error" title="会话读取失败">{errorText(conversations.error)} <Button variant="outline" onClick={() => void conversations.refetch()}>重试</Button></ConsoleState> :
      mode !== "home" ? <section className={styles.list} aria-label="会话列表">
        {conversations.isError ? <ConsoleState kind="error" title="部分会话读取失败">{errorText(conversations.error)} <Button variant="outline" onClick={() => void (conversations.isFetchNextPageError ? conversations.fetchNextPage() : conversations.refetch())}>重试</Button></ConsoleState> : null}
        {visible.length ? visible.map(item => <ConversationLink key={item.ID} item={item} />) : <ConsoleState kind="empty" title={mode === "saved" ? "暂无收藏会话" : mode === "archived" ? "暂无归档会话" : "暂无会话"}>{mode === "archived" ? "已归档会话会出现在这里，可打开详情恢复。" : "新建会话后，会保存到当前账号。"}</ConsoleState>}
        {conversations.hasNextPage ? <Button variant="outline" disabled={conversations.isFetchingNextPage} onClick={() => void conversations.fetchNextPage()}>加载更早会话</Button> : null}</section> : null}
  </>;
}

function ConversationLink({ item }: { item: AIConversation }) {
  return <Link className={styles.conversationLink} href={`/workbench/ai/chat/${item.ID}`} prefetch={false}>
    <span><strong>{item.Title || "未命名会话"}</strong><small>{item.Favorite ? "★ 收藏 · " : ""}{new Date(item.UpdatedAt).toLocaleString("zh-CN")}</small></span><span>打开 →</span>
  </Link>;
}

function ConversationDetail({ scope, authorizationKey, canUse, canPlan, planningReadiness, titleReadiness, id }: { scope: AIScope; authorizationKey: string; canUse: boolean; canPlan: boolean; planningReadiness: "AVAILABLE" | "NEEDS_CONFIGURATION" | "UNAVAILABLE"; titleReadiness: "AVAILABLE" | "NEEDS_CONFIGURATION" | "UNAVAILABLE"; id: string }) {
  const router = useRouter();
  const search = useSearchParams();
  const [operationId, setOperationId] = useState(search.get("operationId") ?? "");
  const [platform, setPlatform] = useState<"shein" | "temu" | "amazon">("shein");
  const [content, setContent] = useState("");
  const [editingTitle, setEditingTitle] = useState(false);
  const [titleDraft, setTitleDraft] = useState("");
  const [knowledgeBaseId, setKnowledgeBaseId] = useState("");
  const [templateId, setTemplateId] = useState("");
  const [templateRevision, setTemplateRevision] = useState("");
  const [pendingMessage, setPendingMessage] = useState<PendingMessage | null>(null);
  const [confirmReceipts, setConfirmReceipts] = useState<Record<string, ConfirmationReceipt>>({});
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const messageStorageKey = pendingMessageStorageKey(scope, id);
  const confirmationStorageKey = confirmStorageKey(scope, id);
  const detail = useInfiniteQuery({ queryKey: ["ai-chat", scope.userId, scope.organizationId, authorizationKey, id], initialPageParam: "",
    queryFn: ({ signal, pageParam }) => requestAIWorkbench({ route: "conversation-read", method: "GET", path: `chat/conversations/${id}?limit=50${pageParam ? `&before=${pageParam}` : ""}`, scope, signal }),
    getNextPageParam: page => page.before || undefined,
    retry: false, staleTime: 0, refetchOnWindowFocus: false });
  const newest = detail.data?.pages[0];
  const current = newest?.conversation;
  const messages = detail.data?.pages.slice().reverse().flatMap(page => page.messages) ?? [];
  const latestUser = newest?.messages.filter(m => m.Author === "USER").at(-1)?.Sequence ?? 0;
  const activeProposal = newest?.proposals.find(p => p.sourceSequence === latestUser);
  const abort = useRef<AbortController | null>(null);
  useEffect(() => {
    let active = true;
    queueMicrotask(() => { if (!active) return;
      try {
      const saved = sessionStorage.getItem(messageStorageKey);
      if (!saved) return;
      const value: unknown = JSON.parse(saved);
      if (!value || typeof value !== "object" || !("key" in value) || typeof value.key !== "string" || !isAcquisitionUUID(value.key) || !("body" in value)) throw new Error("invalid pending message");
      const checked = aiMessageBody.safeParse(value.body);
      if (!checked.success) throw new Error("invalid pending body");
      setPendingMessage({ key: value.key, body: checked.data });
      setContent(checked.data.content); setOperationId(checked.data.operationId); setPlatform(checked.data.targetPlatform);
      setTemplateId(checked.data.templateId ?? ""); setTemplateRevision(checked.data.templateRevision ?? ""); setKnowledgeBaseId(checked.data.knowledgeBaseId ?? "");
      } catch { try { sessionStorage.removeItem(messageStorageKey); } catch { /* storage may be unavailable */ } }
    });
    return () => { active = false; };
  }, [messageStorageKey]);
  useEffect(() => {
    let active = true;
    queueMicrotask(() => { if (!active) return;
      try {
      const raw = sessionStorage.getItem(confirmationStorageKey);
      if (!raw) return;
      const values: unknown = JSON.parse(raw);
      if (!values || typeof values !== "object" || Array.isArray(values)) return;
      const safe: Record<string, ConfirmationReceipt> = {};
      for (const [proposalId, receipt] of Object.entries(values).slice(-50)) {
        if (!isAcquisitionUUID(proposalId) || !receipt || typeof receipt !== "object" || !("key" in receipt) ||
          typeof receipt.key !== "string" || !isAcquisitionUUID(receipt.key)) continue;
        const taskId = "taskId" in receipt && typeof receipt.taskId === "string" && isAcquisitionUUID(receipt.taskId) ? receipt.taskId : undefined;
        safe[proposalId] = { key: receipt.key, ...(taskId ? { taskId } : {}) };
      }
      setConfirmReceipts(safe);
      } catch { /* a missing browser store cannot authorize an operation */ }
    });
    return () => { active = false; };
  }, [confirmationStorageKey]);
  useEffect(() => () => abort.current?.abort(), []);
  async function send() {
    const draft = pendingMessage ?? { key: crypto.randomUUID(), body: {
      content, operationId, targetPlatform: platform,
      ...(knowledgeBaseId ? { knowledgeBaseId } : {}), ...(templateId ? { templateId, templateRevision } : {}),
    } };
    const checked = aiMessageBody.safeParse(draft.body);
    if (!checked.success) { setError("请输入有效的商品操作 ID、需求和模板修订号。"); return; }
    const saved = { key: draft.key, body: checked.data };
    setPendingMessage(saved); setPending(true); setError("");
    try { sessionStorage.setItem(pendingMessageStorageKey(scope, id), JSON.stringify(saved)); } catch { /* retain the in-memory receipt */ }
    const controller = new AbortController(); abort.current = controller;
    try {
      const result = await requestAIWorkbench({ route: "message", method: "POST", path: `chat/conversations/${id}/messages`, scope, key: saved.key, signal: controller.signal, body: saved.body });
      if (controller.signal.aborted) return;
      if (result.state === "COMPLETE" || result.state === "FAILED_BEFORE_DISPATCH" || result.state === "PLANNER_INVALID_OUTPUT" || result.state === "PLANNER_UNKNOWN") {
        setPendingMessage(null);
        try { sessionStorage.removeItem(pendingMessageStorageKey(scope, id)); } catch { /* terminal receipt is already displayed */ }
        if (result.state === "COMPLETE") setContent("");
      }
      if (result.state === "FAILED_BEFORE_DISPATCH") setError("规划尚未发送到模型。请检查当前企业的模型配置或联系管理员；修复后可用保留的需求发送新消息。");
      if (result.state === "PLANNER_INVALID_OUTPUT") setError("模型返回的规划内容格式无效；本轮已产生 AI 用量，不会自动重发。可修改保留的需求后发送新消息。");
      if (result.state === "PLANNER_UNKNOWN") setError("模型结果无法确认；本轮不会自动重发。请刷新查看收据。");
      if (result.state === "READY_TO_DISPATCH") setError("本轮仍在处理，请使用相同操作键重试读取结果。");
      await detail.refetch();
    } catch (cause) { if (!controller.signal.aborted) setError(errorText(cause)); }
    finally { setPending(false); abort.current = null; }
  }
  async function confirm(proposal: AIProposal) {
    const key = confirmReceipts[proposal.id]?.key || crypto.randomUUID();
    const receipts = { ...confirmReceipts, [proposal.id]: { key } };
    setConfirmReceipts(receipts);
    try { sessionStorage.setItem(confirmStorageKey(scope, id), JSON.stringify(receipts)); } catch { /* retain the in-memory key */ }
    setPending(true); setError(""); const controller = new AbortController(); abort.current = controller;
    try {
      const receipt = await requestAIWorkbench({ route: "confirm", method: "POST",
        path: `chat/conversations/${id}/proposals/${proposal.id}/confirm`, scope, key, signal: controller.signal });
      if (controller.signal.aborted) return;
      const completed = { ...receipts, [proposal.id]: { key, taskId: receipt.task.id } };
      setConfirmReceipts(completed);
      try { sessionStorage.setItem(confirmStorageKey(scope, id), JSON.stringify(completed)); } catch { /* receipt remains in the Workbench owner */ }
      router.push(`/workbench/ai/tasks/${receipt.task.id}`);
    } catch (cause) { if (!controller.signal.aborted) setError(errorText(cause)); }
    finally { setPending(false); abort.current = null; }
  }
  async function change(property: { title?: string; favorite?: boolean; archived?: boolean }) {
    if (!current) return; setPending(true); setError("");
    try { await requestAIWorkbench({ route: "conversation-metadata", method: "PATCH", path: `chat/conversations/${id}`,
      scope, revision: current.MetadataRevision, body: property }); await detail.refetch(); setEditingTitle(false); }
    catch (cause) { setError(errorText(cause)); }
    finally { setPending(false); }
  }
  if (detail.isPending) return <ConsoleState kind="loading" title="正在读取会话" />;
  if (detail.isError || !current) return <ConsoleState kind="error" title="会话不可用">{errorText(detail.error)} <Button variant="outline" onClick={() => void detail.refetch()}>重试</Button></ConsoleState>;
  return <div className={styles.detail}>
    <div className={styles.toolbar}><strong>{current.Title || "未命名会话"}</strong><span>当前企业：{scope.organizationId}</span>
      {canUse && !editingTitle ? <Button variant="outline" disabled={pending} onClick={() => { setTitleDraft(current.Title); setEditingTitle(true); }}>修改标题</Button> : null}
      {canUse ? <Button variant="outline" disabled={pending} onClick={() => void change({ favorite: !current.Favorite })}>{current.Favorite ? "取消收藏" : "收藏"}</Button> : null}
      {canUse ? <Button variant="outline" disabled={pending} onClick={() => void change({ archived: !current.Archived })}>{current.Archived ? "恢复会话" : "归档"}</Button> : null}
      <Button variant="outline" onClick={() => void detail.refetch()}>刷新</Button></div>
    {canUse && editingTitle ? <div className={styles.toolbar}><label>会话标题 <input value={titleDraft} disabled={pending} onChange={event => setTitleDraft(event.target.value)} /></label>
      <Button disabled={pending || new TextEncoder().encode(titleDraft).length > 256} onClick={() => void change({ title: titleDraft })}>保存标题</Button>
      <Button variant="outline" disabled={pending} onClick={() => setEditingTitle(false)}>取消</Button>
      {new TextEncoder().encode(titleDraft).length > 256 ? <span>标题不能超过 256 字节。</span> : null}</div> : null}
    <div className={styles.messages} aria-label="会话消息">{messages.length ? messages.map(item => <Card key={item.ID} className={item.Author === "USER" ? styles.userMessage : styles.assistantMessage}>
      <small>{item.Author === "USER" ? "我" : "硕米"} · {new Date(item.CreatedAt).toLocaleString("zh-CN")}</small><p>{item.Content}</p></Card>) : <ConsoleState kind="empty" title="开始讨论">先选定已有商品，再描述标题优化目标。</ConsoleState>}
      {detail.hasNextPage ? <Button variant="outline" disabled={detail.isFetchingNextPage} onClick={() => void detail.fetchNextPage()}>{detail.isFetchingNextPage ? "正在加载…" : "加载更早消息"}</Button> : null}</div>
    {activeProposal ? <ProposalCard proposal={activeProposal} scope={scope} canUse={canUse} titleReadiness={titleReadiness} disabled={pending || current.Archived}
      confirmedTaskId={confirmReceipts[activeProposal.id]?.taskId} onConfirm={() => void confirm(activeProposal)} /> : null}
    {error ? <ConsoleState kind="error" title="操作状态">{error} <Button variant="outline" onClick={() => void detail.refetch()}>刷新收据</Button></ConsoleState> : null}
    {!current.Archived && canUse && (canPlan || pendingMessage) ? <Card className={styles.composer}><h2>讨论标题建议</h2><p>只针对已保存的采集商品生成标题建议。确认提案后才会启动 Product Agent。</p>
      <div className={styles.formGrid}><label>采集操作 ID<input disabled={Boolean(pendingMessage)} value={operationId} onChange={e => setOperationId(e.target.value.trim())} placeholder="从已保存商品采集结果复制操作 ID" /></label>
        <label>目标平台<select disabled={Boolean(pendingMessage)} value={platform} onChange={e => setPlatform(e.target.value as typeof platform)}><option value="shein">SHEIN</option><option value="temu">Temu</option><option value="amazon">Amazon</option></select></label>
        <label>模板 ID（可选）<input disabled={Boolean(pendingMessage)} value={templateId} onChange={e => setTemplateId(e.target.value.trim())} /></label>
        <label>模板修订号{templateId ? "" : "（可选）"}<input disabled={Boolean(pendingMessage)} value={templateRevision} onChange={e => setTemplateRevision(e.target.value.trim())} /></label>
        <label>知识库 ID（可选）<input disabled={Boolean(pendingMessage)} value={knowledgeBaseId} onChange={e => setKnowledgeBaseId(e.target.value.trim())} /></label></div>
      <label className={styles.promptLabel}>需求<textarea disabled={Boolean(pendingMessage)} rows={4} value={content} onChange={e => setContent(e.target.value)} placeholder="例如：请优化这个商品在 SHEIN 的标题，突出已有证据支持的卖点。" /></label>
      {pendingMessage ? <p>原消息结果待确认，字段已锁定；同键重试先核对收据，尚未受理的请求可能在配置就绪后首次发送。</p> : null}
      <div className={styles.toolbar}><Button disabled={pending || !isAcquisitionUUID(operationId) || !content.trim() || Boolean(templateId) !== Boolean(templateRevision)} onClick={() => void send()}>{pending ? "处理中…" : pendingMessage ? "重试同一消息" : "发送消息"}</Button>
        <span>模型规划可能消耗 AI 额度；不会直接修改商品。</span></div></Card> : current.Archived ? <ConsoleState kind="unavailable" title="会话已归档">已保存的任务仍可从任务中心查看。</ConsoleState> :
      <ConsoleState kind="unavailable" title="只读会话">{canUse ? planningStatus(planningReadiness) : "当前企业没有发送或确认权限，可查看已有对话和提案。"}</ConsoleState>}
  </div>;
}

function ProposalCard({ proposal, scope, canUse, titleReadiness, disabled, confirmedTaskId, onConfirm }: { proposal: AIProposal; scope: AIScope; canUse: boolean; titleReadiness: "AVAILABLE" | "NEEDS_CONFIGURATION" | "UNAVAILABLE"; disabled: boolean; confirmedTaskId?: string; onConfirm: () => void }) {
  return <Card className={styles.proposal}><div className={styles.proposalHead}><span>待确认的执行方案</span><strong>仅生成标题建议</strong></div>
    <p>{proposal.goalSummary}</p><dl><dt>企业</dt><dd>{scope.organizationId}</dd><dt>商品</dt><dd>{proposal.detailsAvailable ? proposal.productKey : "当前无权查看"}</dd>
      <dt>目标平台</dt><dd>{proposal.targetPlatform ?? "不可用"}</dd><dt>模板</dt><dd>{proposal.templateId ? `${proposal.templateId} · 修订 ${proposal.templateRevision}` : "未选择"}</dd>
      <dt>知识库</dt><dd>{proposal.knowledgeBaseId || "未选择"}</dd><dt>执行模型</dt><dd>{proposal.providerId && proposal.modelId ? `${proposal.providerId} / ${proposal.modelId}` : "不可用"}</dd>
      <dt>最大额度</dt><dd>{proposal.maximumTokens !== undefined ? `${proposal.maximumTokens} tokens` : "不可用"}{proposal.maximumCostMicros !== undefined ? ` · ${proposal.maximumCostMicros} μ${proposal.currency ?? ""}` : ""}</dd>
      <dt>结果处理</dt><dd>必须人工审核，再由授权人员应用</dd></dl>
    {!confirmedTaskId && canUse && proposal.detailsAvailable && titleReadiness !== "AVAILABLE" ? <p>标题执行模型需要配置；当前方案暂不能确认。</p> : null}
    {!confirmedTaskId && canUse && proposal.detailsAvailable && titleReadiness === "AVAILABLE" && !proposal.titleProfileReady ? <p>提案的标题模型配置已变化，请重新提出方案。</p> : null}
    {confirmedTaskId ? <Button asChild><Link href={`/workbench/ai/tasks/${confirmedTaskId}`} prefetch={false}>查看已确认任务</Link></Button> :
      canUse ? <Button disabled={disabled || !proposal.detailsAvailable || titleReadiness !== "AVAILABLE" || !proposal.titleProfileReady} onClick={onConfirm}>确认并创建任务</Button> : null}
  </Card>;
}
