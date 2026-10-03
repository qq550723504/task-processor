"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ConsolePage, ConsoleState } from "../console/console-page";
import { requestAIWorkbench, AIWorkbenchError, type AIScope } from "@/lib/api/ai-workbench";
import type { AITask } from "@/lib/contracts/ai-workbench";
import { findConsoleRoute } from "@/lib/workbench/console-navigation";
import styles from "./task-page.module.css";

type TaskMode = "all" | "running" | "pending" | "completed" | "errors";
const filters: { mode: TaskMode; label: string; href: string }[] = [
  { mode: "all", label: "全部", href: "/workbench/ai/tasks" },
  { mode: "pending", label: "待确认", href: "/workbench/ai/tasks/pending" },
  { mode: "running", label: "执行中", href: "/workbench/ai/tasks/running" },
  { mode: "completed", label: "已完成", href: "/workbench/ai/tasks/completed" },
  { mode: "errors", label: "异常任务", href: "/workbench/ai/tasks/errors" },
];
const taskState: Record<string, string> = { RUNNING: "执行中", WAITING_CONFIRMATION: "待确认", COMPLETED: "已完成", ERROR: "异常", PAUSED: "已暂停" };
const reasonText: Record<string, string> = { START_NOT_CLAIMED: "执行尚未启动，可使用原任务启动", EXECUTION_OUTCOME_UNKNOWN: "执行结果无法确认，不会自动重发模型请求",
  HUMAN_REVIEW_REQUIRED: "需要人工审核标题建议", pending: "提案待审核", accepted: "提案已接受，待应用", applied: "提案已应用", rejected: "提案已拒绝" };
const errorText = (error: unknown) => error instanceof AIWorkbenchError ? `请求未完成：${error.code}` : "任务服务暂不可用";

export function BusinessTaskPage({ mode = "all", taskId }: { mode?: TaskMode; taskId?: string }) {
  const context = useWorkbenchContext();
  const scope = context.user && context.effectiveOrganization && !context.selectionRequired && !context.isSwitching && !context.error && !context.blockingError
    ? { userId: context.user.id, organizationId: context.effectiveOrganization.id } : null;
  const authorizationKey = context.roles.join(",");
  const path = taskId ? "/workbench/ai/tasks" : filters.find(item => item.mode === mode)?.href ?? "/workbench/ai/tasks";
  return <ConsolePage title={taskId ? "任务详情" : "任务中心"} breadcrumbs={findConsoleRoute(path)?.trail}
    description="查看已确认的业务任务及其当前执行和审核状态。状态来自 Product Agent 与 Review 的实时投影。">
    {!scope ? <ConsoleState kind={context.isLoading || context.isSwitching ? "loading" : "unavailable"} title="企业上下文不可用">请选择可访问的企业并登录。</ConsoleState> :
      <ScopedTasks key={`${scope.userId}:${scope.organizationId}:${authorizationKey}`} scope={scope} authorizationKey={authorizationKey} mode={mode} taskId={taskId} />}
  </ConsolePage>;
}

function ScopedTasks({ scope, authorizationKey, mode, taskId }: { scope: AIScope; authorizationKey: string; mode: TaskMode; taskId?: string }) {
  if (taskId) return <TaskDetail scope={scope} authorizationKey={authorizationKey} taskId={taskId} />;
  return <TaskList scope={scope} authorizationKey={authorizationKey} mode={mode} />;
}

function TaskList({ scope, authorizationKey, mode }: { scope: AIScope; authorizationKey: string; mode: TaskMode }) {
  const [after, setAfter] = useState("");
  const tasks = useQuery({ queryKey: ["ai-tasks", scope.userId, scope.organizationId, authorizationKey, after],
    queryFn: ({ signal }) => requestAIWorkbench({ route: "task-list", method: "GET", path: `tasks?limit=50${after ? `&after=${after}` : ""}`, scope, signal }),
    retry: false, staleTime: 0, refetchOnWindowFocus: false });
  const page = tasks.data?.tasks ?? [];
  const visible = page.filter(item => mode === "all" || mode === "pending" && item.state === "WAITING_CONFIRMATION" ||
    mode === "running" && item.state === "RUNNING" || mode === "completed" && item.state === "COMPLETED" ||
    mode === "errors" && (item.state === "ERROR" || item.state === "PAUSED"));
  return <>
    <div className={styles.metrics}>
      {[["待确认", "WAITING_CONFIRMATION"], ["执行中", "RUNNING"], ["已完成", "COMPLETED"]].map(([label, state]) => <Card key={state} className={styles.metric}>
        <span>{label}</span><strong>{page.filter(item => item.state === state).length}</strong><small>当前页</small></Card>)}
      <Card className={styles.scope}><span>工作范围</span><strong>{scope.organizationId}</strong><small>当前企业 · 当前账号</small></Card>
    </div>
    <div className={styles.toolbar}><nav aria-label="任务状态" className={styles.filters}>{filters.map(item => <Link key={item.mode} href={item.href} aria-current={mode === item.mode ? "page" : undefined} prefetch={false}>{item.label}</Link>)}</nav>
      <Button variant="outline" onClick={() => void tasks.refetch()}>刷新状态</Button><Button asChild><Link href="/workbench/ai/chat/new" prefetch={false}>向硕米发起任务</Link></Button></div>
    {tasks.isPending ? <ConsoleState kind="loading" title="正在读取任务" /> : tasks.isError ? <ConsoleState kind="error" title="任务读取失败">{errorText(tasks.error)} <Button variant="outline" onClick={() => void tasks.refetch()}>重试</Button></ConsoleState> :
      <div className={styles.list}>{visible.length ? visible.map(item => <TaskRow key={item.id} task={item} />) : <ConsoleState kind="empty" title="当前筛选无任务">只显示通过 Chat 提案确认创建的 BusinessTask。</ConsoleState>}
        {tasks.data?.next ? <Button variant="outline" onClick={() => setAfter(tasks.data!.next)}>加载更早任务</Button> : null}</div>}
    {mode === "pending" ? <Card className={styles.secondary}><h2>标准商品标题审核</h2><p>查看 Product Review 持有的全部标题提案，包括从 BusinessTask 提交和直接从 Product Agent 提交的提案。接受后仍需单独应用。</p><Button asChild variant="outline"><Link href="/workbench/ai/tasks/pending/other" prefetch={false}>打开标题审核</Link></Button></Card> : null}
    {mode === "completed" ? <Card className={styles.secondary}><h2>历史已完成工作记录</h2><p>原本地资料准备记录继续单独展示，不计入 BusinessTask。</p><Button asChild variant="outline"><Link href="/workbench/ai/tasks/completed/history" prefetch={false}>查看历史记录</Link></Button></Card> : null}
  </>;
}

function TaskRow({ task }: { task: AITask }) {
  return <Link className={styles.row} href={`/workbench/ai/tasks/${task.id}`} prefetch={false}>
    <span className={styles.rowAccent} data-state={task.state ?? "unknown"} /><span><strong>{task.title}</strong><small>{task.productDetailsAvailable ? `${task.productKey} · ${task.targetPlatform}` : "商品详情暂不可用"}</small><small>{task.projectionAvailable ? reasonText[task.reason ?? ""] ?? task.reason ?? "" : "状态暂不可用"}</small></span>
    <span className={styles.badge}>{task.projectionAvailable ? taskState[task.state ?? ""] ?? "状态未知" : "状态暂不可用"}</span><span className={styles.rowLink}>查看详情 →</span>
  </Link>;
}

function TaskDetail({ scope, authorizationKey, taskId }: { scope: AIScope; authorizationKey: string; taskId: string }) {
  const [pending, setPending] = useState(false);
  const [feedback, setFeedback] = useState("");
  const [error, setError] = useState("");
  const abort = useRef<AbortController | null>(null);
  useEffect(() => () => abort.current?.abort(), []);
  const task = useQuery({ queryKey: ["ai-task", scope.userId, scope.organizationId, authorizationKey, taskId],
    queryFn: ({ signal }) => requestAIWorkbench({ route: "task-read", method: "GET", path: `tasks/${taskId}`, scope, signal }),
    retry: false, staleTime: 0, refetchOnWindowFocus: false });
  const item = task.data?.task;
  async function act(action: "start" | "resume" | "review") {
    if (!item) return; setPending(true); setError("");
    const controller = new AbortController(); abort.current = controller;
    try {
      await requestAIWorkbench({ route: `task-${action}`, method: "POST", path: `tasks/${taskId}/${action}`, scope,
        signal: controller.signal, ...(action === "resume" ? { body: { revision: item.agentRevision, feedback } } : {}) });
      if (!controller.signal.aborted) await task.refetch();
    } catch (cause) { if (!controller.signal.aborted) setError(errorText(cause)); }
    finally { if (!controller.signal.aborted) setPending(false); abort.current = null; }
  }
  if (task.isPending) return <ConsoleState kind="loading" title="正在读取任务" />;
  if (task.isError || !item) return <ConsoleState kind="error" title="任务不可用">{errorText(task.error)} <Button variant="outline" onClick={() => void task.refetch()}>重试</Button></ConsoleState>;
  return <div className={styles.detail}>
    <div className={styles.toolbar}><Button asChild variant="outline"><Link href="/workbench/ai/tasks" prefetch={false}>返回任务中心</Link></Button><Button variant="outline" onClick={() => void task.refetch()}>刷新状态</Button></div>
    <Card className={styles.detailCard}><div className={styles.heading}><h2>{item.title}</h2><span className={styles.badge}>{item.projectionAvailable ? taskState[item.state ?? ""] ?? "状态未知" : "状态暂不可用"}</span></div>
      <p>{item.goalSummary}</p><dl><dt>当前状态</dt><dd>{item.projectionAvailable ? reasonText[item.reason ?? ""] ?? item.reason ?? "" : "Owner 暂不可用，无法确认当前状态"}</dd>
        <dt>企业</dt><dd>{scope.organizationId}</dd><dt>商品</dt><dd>{item.productDetailsAvailable ? item.productKey : "当前无权查看"}</dd>
        <dt>目标平台</dt><dd>{item.productDetailsAvailable ? item.targetPlatform : "不可用"}</dd>
        <dt>执行模型</dt><dd>{item.providerId && item.modelId ? `${item.providerId} / ${item.modelId}` : "尚无可用执行记录"}</dd>
        <dt>用量</dt><dd>{item.usageStatus === "unknown_reserved" ? "结果未知，额度仍保留" : item.usageStatus === "observed" ? `${item.tokens ?? 0} tokens · ${item.estimatedCostMicros ?? 0} μ${item.currency ?? ""}` : "尚无可用用量"}</dd>
        <dt>审核</dt><dd>{item.reviewState || "尚未生成"}</dd>
        <dt>创建时间</dt><dd>{new Date(item.createdAt).toLocaleString("zh-CN")}</dd></dl>
      {item.productDetailsAvailable && item.operationId ? <Button asChild variant="outline"><Link href={`/workbench/supply/acquisition/operation/${item.operationId}`} prefetch={false}>查看来源商品</Link></Button> : null}
    </Card>
    {error ? <ConsoleState kind="error" title="操作未完成">{error}。请刷新原任务查看状态，不要新建任务。</ConsoleState> : null}
    <div className={styles.actions}>
      {item.canStart ? <Button disabled={pending} onClick={() => void act("start")}>启动原任务</Button> : null}
      {item.canResume ? <Card className={styles.actionCard}><label>继续执行的反馈<textarea rows={3} value={feedback} onChange={event => setFeedback(event.target.value)} /></label><Button disabled={pending || !feedback.trim()} onClick={() => void act("resume")}>继续原任务</Button></Card> : null}
      {item.canReview && !item.reviewId ? <Button disabled={pending} onClick={() => void act("review")}>提交标题供人工审核</Button> : null}
      {item.reviewId ? <Button asChild><Link href={`/workbench/ai/tasks/pending/other?proposal_id=${item.reviewId}`} prefetch={false}>查看并处理标题提案</Link></Button> : null}
    </div>
  </div>;
}
