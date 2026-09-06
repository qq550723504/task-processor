"use client";

import { ConsolePage } from "@/components/workbench/console/console-page";
import { useQuery } from "@tanstack/react-query";
import { RefreshCw, ShieldCheck } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Select } from "@/components/ui/select";
import type { SheinDiagnosticAction } from "@/lib/api/shein-diagnostic";
import { fetchSheinDiagnostic } from "@/lib/api/shein-diagnostic-client";

import { DiagnosticReport } from "./diagnostic-report";

export function SheinDiagnosticPage({ recordId }: { recordId: string }) {
  const context = useWorkbenchContext();
  if (context.isSwitching) return <ContextState message="正在切换企业，已清空诊断结果…" />;
  if (context.error || context.blockingError || !context.user || !context.effectiveOrganization || context.selectionRequired || context.isLoading) {
    return <ContextState message="企业或登录上下文不可用，已停止加载诊断。" />;
  }
  // A context transition destroys the entire request subtree, even when the
  // transport cannot cancel. Neither prior data nor action/digest state survives.
  const scope = JSON.stringify([context.user.id, context.effectiveOrganization.id, context.roles, recordId]);
  return <ScopedDiagnostic key={scope} recordId={recordId} organizationId={context.effectiveOrganization.id} organizationName={context.effectiveOrganization.name} scope={scope} />;
}

function ScopedDiagnostic({ recordId, organizationId, organizationName, scope }: { recordId: string; organizationId: string; organizationName: string; scope: string }) {
  const [request, setRequest] = useState<{ action: SheinDiagnosticAction; sequence: number; expectedDigest?: string }>({ action: "publish", sequence: 0 });
  function check(action: SheinDiagnosticAction, expectedDigest?: string) {
    setRequest((previous) => ({ action, sequence: previous.sequence + 1, ...(expectedDigest ? { expectedDigest } : {}) }));
  }
  return (
    <ConsolePage title="SHEIN 资料诊断" breadcrumbs={[{ label: "SHEIN 资料诊断" }]} description={organizationName}>
      <div className="mb-6">

        <p className="mt-3 flex items-center gap-2 text-sm font-medium"><ShieldCheck aria-hidden="true" className="size-4 shrink-0" />本地资料 / 仅离线检查</p>
        <p className="mt-2 max-w-2xl text-sm leading-6 text-foreground/75">这是硕米保存的本地资料，不是已保存到 SHEIN 的商品。检查不会修改资料或向平台提交。</p>
      </div>
      <Card className="mb-5 p-4 sm:p-5">
        <label className="grid max-w-sm gap-2 text-sm font-medium">
          检查动作
          <Select value={request.action} onChange={(event) => check(event.target.value as SheinDiagnosticAction)}>
            <option value="publish">按发布规则检查</option>
            <option value="save_draft">按本地草稿规则检查</option>
          </Select>
        </label>
        <p className="mt-2 text-xs leading-5 text-muted-foreground">切换后读取该动作的当次结果；两种动作都只检查，不执行发布或保存。</p>
      </Card>
      <DiagnosticRequest key={`${request.action}:${request.sequence}`} recordId={recordId} organizationId={organizationId} scope={scope} request={request} check={check} />
    </ConsolePage>
  );
}

function DiagnosticRequest({ recordId, organizationId, scope, request, check }: { recordId: string; organizationId: string; scope: string; request: { action: SheinDiagnosticAction; sequence: number; expectedDigest?: string }; check: (action: SheinDiagnosticAction, expectedDigest?: string) => void }) {
  const result = useQuery({
    queryKey: ["workbench", organizationId, "shein-diagnostic", recordId, request.action, scope, request.sequence],
    queryFn: ({ signal }) => fetchSheinDiagnostic({ recordId, organizationId, action: request.action, expectedDigest: request.expectedDigest, signal }),
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    refetchInterval: false,
  });

  if (result.isPending || result.isFetching) {
    return <Card className="p-6" role="status" aria-live="polite">正在检查，请稍候…</Card>;
  }
  if (result.isError) return <DiagnosticError error={result.error} retry={() => check(request.action)} />;
  if (!result.data) return <DiagnosticError error={{ code: "INVALID_UPSTREAM_RESPONSE" }} retry={() => check(request.action)} />;
  return (
    <>
      <div className="mb-4 flex flex-wrap gap-3">
        <Button onClick={() => check(request.action)}><RefreshCw aria-hidden="true" />重新检查</Button>
        <Button variant="outline" onClick={() => check(request.action, result.data.input.actual_digest)}>复核同一内容</Button>
        <p className="w-full text-xs text-foreground/75">重新检查读取当前内容；同内容复核会核对本次内容摘要，发生变化时提示。</p>
      </div>
      <DiagnosticReport report={result.data} recordId={recordId} />
    </>
  );
}

function DiagnosticError({ error, retry }: { error: unknown; retry: () => void }) {
  const context = useWorkbenchContext();
  const mounted = useRef(false);
  const [recovering, setRecovering] = useState(false);
  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);
  async function recoverContext() {
    setRecovering(true);
    const recovered = await context.retry();
    // Action/context changes unmount this error. A late recovery must not
    // restart the request that the user already left behind.
    if (!mounted.current) return;
    setRecovering(false);
    const recoveredOrganization = recovered?.organizations.find((organization) => organization.id === recovered.effectiveOrganizationId);
    if (recovered && !recovered.selectionRequired && recovered.user.id === context.user?.id &&
      recoveredOrganization?.id === context.effectiveOrganization?.id &&
      JSON.stringify(recoveredOrganization?.roles) === JSON.stringify(context.roles)) retry();
  }
  const code = error && typeof error === "object" && "code" in error && typeof error.code === "string" ? error.code : "REQUEST_FAILED";
  const messages: Record<string, string> = {
    permission_denied: "没有读取这份资料的权限",
    PERMISSION_DENIED: "没有读取这份资料的权限",
    AUTHENTICATION_REQUIRED: "登录状态已失效",
    not_found: "资料不存在或当前账号不可读取",
    stale_input: "资料内容或有效性已变化",
    ORGANIZATION_CONTEXT_CHANGED: "企业上下文已变化",
    ORGANIZATION_ACCESS_REVOKED: "当前企业访问已撤销",
    ORGANIZATION_ACCESS_DENIED: "当前企业访问被拒绝",
    ORGANIZATION_SUSPENDED: "当前企业已暂停访问",
    deadline_exceeded: "检查超时",
    DEADLINE_EXCEEDED: "检查超时",
    INVALID_UPSTREAM_RESPONSE: "诊断响应不合法",
    invalid_input: "资料内容不符合诊断合同",
    input_too_large: "资料超过检查大小限制",
    invalid_request: "资料链接或检查参数不合法",
    unsupported_action: "检查动作不受支持",
    unsupported_rule_version: "诊断规则版本不受支持",
    evaluation_failed: "诊断服务未能完成评估",
    DEPENDENCY_UNAVAILABLE: "诊断服务暂不可用",
    unavailable: "诊断服务暂不可用",
  };
  const changed = code === "stale_input";
  const contextError = code.startsWith("ORGANIZATION_") || code === "AUTHENTICATION_REQUIRED";
  return (
    <Card className="p-5 sm:p-6">
      <div role="alert">
        <h2 className="font-semibold">{messages[code] || "诊断请求失败，请稍后重试"}</h2>
        <p className="mt-2 text-sm text-muted-foreground">{changed ? "内容绑定或外部资料有效性检查未通过，未自动重试。可明确选择检查当前内容，获取新的诊断。" : "本次未取得可用诊断，旧结果已隐藏。"}</p>
      </div>
      <Button className="mt-4" variant="outline" disabled={recovering} onClick={contextError ? recoverContext : retry}>{recovering ? "正在恢复企业上下文…" : contextError ? "重新加载企业上下文" : changed ? "检查当前内容" : "重新检查"}</Button>
    </Card>
  );
}

function ContextState({ message }: { message: string }) {
  return <p className="p-6 text-sm text-foreground/75" role="status">{message}</p>;
}
