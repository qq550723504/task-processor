"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { applyProductTitleProposal, decideProductTitleProposal, fetchProductTitleProposal, fetchProductTitleProposals, ProductTitleReviewError } from "@/lib/api/product-title-review-client";
import { ConsoleState } from "../console/console-page";
import { TaskCenterLayout } from "./task-center-layout";
import { TitleReviewPanels } from "./title-review-panels";
import { TitleReviewControls, TitleReviewDetail, titleReviewStateLabel } from "./title-review-detail";

const queryPolicy = { gcTime: 0, staleTime: 0, retry: false, refetchOnWindowFocus: false, refetchOnReconnect: false, refetchInterval: false } as const;
const errorCode = (error: unknown) => error instanceof ProductTitleReviewError ? error.code : "DEPENDENCY_UNAVAILABLE";
const denied = (error: unknown) => error instanceof ProductTitleReviewError && (error.status === 401 || error.status === 403 || error.code.startsWith("ORGANIZATION_"));

export function PendingTitleReviewPageContent({ available, initialProposalId }: { available: boolean; initialProposalId?: string }) {
  const context = useWorkbenchContext();
  const valid = !context.isSwitching && !context.isLoading && !context.error && !context.blockingError && !context.selectionRequired && context.user && context.effectiveOrganization;
  const scope = valid ? JSON.stringify([context.user!.id, context.effectiveOrganization!.id, context.roles]) : "unavailable";
  const [entry, setEntry] = useState<{ scope: string | null; id?: string }>({ scope: valid ? scope : null, id: initialProposalId });
  // A changed identity/context permanently clears the initial URL selection,
  // including A -> unavailable -> A. The next selection requires a user action.
  if (valid && entry.scope !== scope) setEntry({ scope, id: entry.scope === null ? entry.id : undefined });
  if (!valid && entry.scope !== null && entry.scope !== "unavailable") setEntry({ scope: "unavailable", id: undefined });
  if (!available) return <TaskCenterLayout pendingReview><ConsoleState kind="unavailable" title="标题审核暂未启用">当前环境尚未配置标题提案服务，未读取业务数据。</ConsoleState></TaskCenterLayout>;
  if (!valid) return <TaskCenterLayout pendingReview><ConsoleState kind={context.isSwitching ? "loading" : "unavailable"} title={context.isSwitching ? "正在切换企业" : "企业或登录上下文不可用"}>已清空提案、编辑和操作状态；停止等待不撤销服务器提交。</ConsoleState></TaskCenterLayout>;
  return <ScopedReviews key={scope} scope={scope} organizationId={context.effectiveOrganization!.id} userId={context.user!.id} roles={context.roles} initialProposalId={entry.scope === scope ? entry.id : undefined} />;
}

function ScopedReviews({ scope, organizationId, userId, roles, initialProposalId }: { scope: string; organizationId: string; userId: string; roles: string[]; initialProposalId?: string }) {
  const [selected, setSelected] = useState(initialProposalId);
  const [position, setPosition] = useState({ cursors: [undefined] as (string | undefined)[], page: 1, sequence: 0 });
  const [locked, setLocked] = useState(false);
  const [accessError, setAccessError] = useState<unknown>();
  const response = useQuery({ ...queryPolicy, queryKey: ["workbench", scope, "title-proposals", position], enabled: !accessError,
    queryFn: ({ signal }) => fetchProductTitleProposals({ organizationId, limit: 20, cursor: position.cursors.at(-1), signal }) });
  if (denied(response.error) && accessError !== response.error) {
    setAccessError(response.error); setLocked(false); setSelected(undefined);
  }
  function select(id?: string) {
    if (locked) return;
    setSelected(id);
    const url = new URL(window.location.href); if (id) url.searchParams.set("proposal_id", id); else url.searchParams.delete("proposal_id");
    window.history.replaceState(null, "", url);
  }
  function refresh() {
    if (locked) return;
    select(); setPosition((old) => ({ cursors: [undefined], page: 1, sequence: old.sequence + 1 }));
  }
  return <TaskCenterLayout pendingReview onRefresh={refresh} refreshDisabled={locked}>
    {accessError ? <ReviewError error={accessError} recover={() => { setAccessError(undefined); refresh(); }} /> : <TitleReviewPanels
      entries={(response.data?.items ?? []).map((item) => ({ key: item.proposal_id, heading: "调整标准商品标题", caption: item.product_key, statusLabel: titleReviewStateLabel[item.state], accepted: item.state === "accepted", metadata: <span>基线 {item.base_version} · 提案修订 {item.proposal_revision}</span> }))}
      selectedKey={selected} onSelect={select} onClose={() => select()} busy={locked}
      listState={response.isPending || response.isFetching ? <ConsoleState kind="loading" title="正在读取标题提案">正在校验当前授权范围。</ConsoleState> : response.error ? <ReviewError error={response.error} /> : undefined}
      pagination={<div className="mb-4 flex flex-wrap items-center gap-3 text-xs text-muted-foreground"><span role="status">第 {position.page} 页</span>
        <Button variant="outline" size="sm" disabled={locked || response.isFetching || position.cursors.length <= 1} onClick={() => { select(); setPosition((old) => ({ cursors: old.cursors.slice(0, -1), page: old.page - 1, sequence: old.sequence + 1 })); }}>上一页</Button>
        <Button variant="outline" size="sm" disabled={locked || response.isFetching || !response.data?.next_cursor} onClick={() => { const cursor = response.data?.next_cursor; if (cursor) { select(); setPosition((old) => ({ cursors: [...old.cursors, cursor].slice(-50), page: old.page + 1, sequence: old.sequence + 1 })); } }}>下一页</Button>
        <p className="w-full">每页最多 20 条，按提案标识排序；刷新重新读取，不表示全局待办数量。</p></div>}
      detail={selected ? <ProposalSession key={selected} proposalId={selected} organizationId={organizationId} scope={scope} userId={userId} roles={roles}
        onLocked={setLocked} onDenied={(err) => { setLocked(false); setAccessError(err); setSelected(undefined); }} onChanged={() => { void response.refetch(); }} /> : undefined}
    />}
  </TaskCenterLayout>;
}

type Intent = { kind: "decision"; request: Omit<Parameters<typeof decideProductTitleProposal>[0], "signal"> } | { kind: "apply"; request: Omit<Parameters<typeof applyProductTitleProposal>[0], "signal"> };
function ProposalSession({ proposalId, organizationId, scope, userId, roles, onLocked, onDenied, onChanged }: {
  proposalId: string; organizationId: string; scope: string; userId: string; roles: string[];
  onLocked: (locked: boolean) => void; onDenied: (error: unknown) => void; onChanged: () => void;
}) {
  const [read, setRead] = useState(0);
  const queryClient = useQueryClient();
  const [writeError, setWriteError] = useState<unknown>();
  const [pending, setPending] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  const operation = useRef<Intent | undefined>(undefined);
  const writing = useRef(false);
  const mounted = useRef(false);
  const abort = useRef<AbortController | undefined>(undefined);
  const queryKey = ["workbench", scope, "title-proposal", proposalId, read];
  const query = useQuery({ ...queryPolicy, queryKey, queryFn: ({ signal }) => fetchProductTitleProposal({ organizationId, proposalId, signal }) });
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; abort.current?.abort(); operation.current = undefined; }; }, []);
  useEffect(() => { if (query.isError && denied(query.error)) onDenied(query.error); }, [query.isError, query.error, onDenied]);
  const data = query.data;
  function reread() { if (writing.current) return; if (!uncertain) setWriteError(undefined); setRead((old) => old + 1); }
  async function submit(intent: Intent) {
    if (writing.current) return;
    writing.current = true; operation.current = intent; setPending(true); onLocked(true); setWriteError(undefined);
    const controller = new AbortController(); abort.current = controller;
    try {
      const next = intent.kind === "decision" ? await decideProductTitleProposal({ ...intent.request, signal: controller.signal }) : await applyProductTitleProposal({ ...intent.request, signal: controller.signal });
      if (!mounted.current) return;
      operation.current = undefined; setUncertain(false); onLocked(false);
      if (uncertain) setRead((old) => old + 1); else queryClient.setQueryData(queryKey, next);
      onChanged();
    } catch (error) {
      if (!mounted.current) return;
      if (denied(error)) { operation.current = undefined; onDenied(error); return; }
      const unknown = uncertain || !(error instanceof ProductTitleReviewError) || error.outcome === "unknown";
      setUncertain(unknown); setWriteError(error); onLocked(unknown);
      if (!unknown) operation.current = undefined;
    } finally { if (mounted.current) { writing.current = false; setPending(false); } }
  }
  function decide(action: "accept" | "edit" | "reject", title?: string) {
    if (!data || uncertain || writing.current || query.isFetching) return;
    void submit({ kind: "decision", request: { organizationId, proposalId, idempotencyKey: crypto.randomUUID(), input: action === "edit" ? { action, expected_revision: data.revision, title: title ?? "" } : { action, expected_revision: data.revision } } });
  }
  function apply() {
    if (!data || data.state !== "accepted" || uncertain || writing.current || query.isFetching) return;
    void submit({ kind: "apply", request: { organizationId, proposalId, idempotencyKey: crypto.randomUUID(), input: { expected_revision: data.revision } } });
  }
  const conflict = writeError && ["stale_product_version", "operation_conflict"].includes(errorCode(writeError)) && !uncertain;
  return <>
    {uncertain ? <ConsoleState kind="error" title="结果待核实"><p>请求可能已经提交。重新读取未看到回执也不能证明没有提交。只能显式核实本次操作。</p><Button variant="outline" disabled={pending || query.isFetching} onClick={() => { if (operation.current) void submit(operation.current); }}>核实本次操作</Button></ConsoleState> : null}
    {pending ? <p role="status" className="my-4 text-sm">正在提交；停止等待不撤销服务器事实。<Button variant="outline" onClick={() => abort.current?.abort()}>停止等待</Button></p> : null}
    <Button variant="outline" className="mt-3" disabled={pending || query.isFetching} onClick={reread}>重新读取当前状态</Button>
    {query.isPending || query.isFetching ? <ConsoleState kind="loading" title="正在读取提案详情">重新校验访问权限和当前修订。</ConsoleState>
      : query.isError ? <ReviewError error={query.error} recover={reread} />
      : conflict ? <ReviewError error={writeError} recover={reread} />
      : data ? <>
        {writeError && !uncertain ? <ReviewError error={writeError} /> : null}
        {!uncertain ? <TitleReviewControls key={`${data.revision}:${read}`} proposal={data} roles={roles} userId={userId} pending={pending} decide={decide} apply={apply} /> : null}
        <TitleReviewDetail proposal={data} />
      </> : null}
  </>;
}

function ReviewError({ error, recover }: { error: unknown; recover?: () => void }) {
  const context = useWorkbenchContext();
  const alive = useRef(false);
  const [recovering, setRecovering] = useState(false);
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  const messages: Record<string, string> = {
    PERMISSION_DENIED: "当前身份没有标题审核权限", permission_denied: "当前身份没有标题审核权限",
    AUTHENTICATION_REQUIRED: "登录状态已失效", ORGANIZATION_ACCESS_REVOKED: "当前企业访问已撤销", ORGANIZATION_ACCESS_DENIED: "当前企业访问被拒绝",
    ORGANIZATION_SUSPENDED: "当前企业已暂停访问", ORGANIZATION_CONTEXT_CHANGED: "企业上下文已变化", ORGANIZATION_SELECTION_REQUIRED: "请重新选择企业",
    stale_product_version: "资料已变化，请重新读取", operation_conflict: "资料已变化，请重新读取",
    INVALID_REQUEST: "请求内容不合法，请检查标题与提案标识", invalid_request: "请求内容不合法，请检查标题与提案标识", input_too_large: "标题或请求内容过长",
    not_found: "提案不存在或当前身份不可读取", INVALID_UPSTREAM_RESPONSE: "提案响应不合法", DEADLINE_EXCEEDED: "读取提案超时", deadline_exceeded: "读取提案超时",
    DEPENDENCY_UNAVAILABLE: "标题提案服务暂不可用", unavailable: "标题提案服务暂不可用",
  };
  async function restore() {
    setRecovering(true); const next = await context.retry();
    if (!alive.current) return; setRecovering(false);
    const org = next?.organizations.find((item) => item.id === next.effectiveOrganizationId);
    if (next && !next.selectionRequired && next.user.id === context.user?.id && org?.id === context.effectiveOrganization?.id && JSON.stringify(org?.roles) === JSON.stringify(context.roles)) recover?.();
  }
  return <ConsoleState kind="error" title={messages[errorCode(error)] ?? "标题提案请求失败"}><p>未取得可用的当前结果。请重新授权读取；不会自动重发写请求。</p>
    {recover && denied(error) ? <Button variant="outline" disabled={recovering} onClick={restore}>重新加载企业上下文</Button> : null}
  </ConsoleState>;
}
