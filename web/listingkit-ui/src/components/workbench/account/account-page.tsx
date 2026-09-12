"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { AccountReadError, getAccountOrganization, getAccountProfile } from "@/lib/api/account";
import { ConsoleState } from "../console/console-page";
import { AccountShell } from "./account-shell";
import { OrganizationView, ProfileView } from "./account-views";
import styles from "./account.module.css";

type PageKind = "profile" | "organization";
export function AccountPage({ page, expectedUserId }: { page: PageKind; expectedUserId: string }) {
  const context = useWorkbenchContext();
  const [leaving, setLeaving] = useState(false);
  useEffect(() => {
    const onClick = (event: MouseEvent) => {
      const link = event.target instanceof Element ? event.target.closest("a") : null;
      if (link && new URL(link.href, window.location.href).pathname === "/api/zitadel-auth/logout") setLeaving(true);
    };
    document.addEventListener("click", onClick, true);
    return () => document.removeEventListener("click", onClick, true);
  }, []);
  const title = page === "profile" ? "账户资料" : "企业空间";
  const authError = [context.error, context.blockingError].some(error => error?.code === "AUTHENTICATION_REQUIRED");
  const identityChanged = !!context.user && context.user.id !== expectedUserId;
  const organization = context.effectiveOrganization;
  // A keyed request subtree discards both visible data and consumed query signals on context changes.
  const scope = JSON.stringify([expectedUserId, context.user?.id, organization?.id, context.roles, context.error?.code, context.blockingError?.code, context.selectionRequired, context.isLoading]);
  let content;
  if (leaving || authError || identityChanged) content = <ReadError page={page} code={identityChanged ? "IDENTITY_CONTEXT_CHANGED" : "AUTHENTICATION_REQUIRED"} />;
  else if (context.isSwitching || (page === "organization" && context.isLoading)) content = <ConsoleState kind="loading" title="正在确认当前上下文">旧资料已清除。</ConsoleState>;
  else if (page === "organization" && (context.error || context.blockingError || !context.user || !organization || context.selectionRequired)) content = <ReadError code={context.blockingError?.code ?? context.error?.code ?? "ORGANIZATION_SELECTION_REQUIRED"} />;
  else content = <ScopedAccount key={`${page}:${scope}`} page={page} scope={scope} expectedUserId={expectedUserId} organizationId={organization?.id} />;
  return <AccountShell pathname={`/workbench/account/${page}`} title={title} description={page === "profile" ? "查看你的账户信息与当前资料状态" : "查看当前企业信息与项目访问权限"}>{content}</AccountShell>;
}

function ScopedAccount({ page, scope, expectedUserId, organizationId }: { page: PageKind; scope: string; expectedUserId: string; organizationId?: string }) {
  const [sequence, setSequence] = useState(0);
  return <><AccountRequest key={sequence} page={page} scope={scope} expectedUserId={expectedUserId} organizationId={organizationId} sequence={sequence} /><div className={styles.refresh}><Button variant="outline" onClick={() => setSequence(value => value + 1)}>刷新资料</Button></div></>;
}
function AccountRequest({ page, scope, expectedUserId, organizationId, sequence }: { page: PageKind; scope: string; expectedUserId: string; organizationId?: string; sequence: number }) {
  const response = useQuery({ queryKey: ["account", page, scope, sequence], queryFn: async ({ signal }) => page === "profile" ? getAccountProfile({ expectedUserId, signal }) : getAccountOrganization({ expectedUserId, expectedOrganizationId: organizationId!, signal }), gcTime: 0, staleTime: 0, retry: false, refetchOnWindowFocus: true, refetchOnReconnect: true });
  if (response.isPending || response.isFetching) return <ConsoleState kind="loading" title="正在读取资料">正在确认当前身份和访问权限。</ConsoleState>;
  if (response.isError) return <ReadError code={response.error instanceof AccountReadError ? response.error.code : "UNKNOWN"} page={page} />;
  return "effectiveOrganizationId" in response.data ? <OrganizationView data={response.data} /> : <ProfileView data={response.data} />;
}
function ReadError({ code, page = "profile" }: { code: string; page?: PageKind }) {
  const messages: Record<string, string> = { AUTHENTICATION_REQUIRED: "登录已失效", IDENTITY_CONTEXT_CHANGED: "登录身份已变化", ACCOUNT_NOT_CONFIGURED: "账户资料服务尚未配置", DEPENDENCY_UNAVAILABLE: "资料服务暂不可用", DEADLINE_EXCEEDED: "资料读取超时", PERMISSION_DENIED: "无查看权限", ORGANIZATION_ACCESS_DENIED: "企业访问被拒绝", ORGANIZATION_ACCESS_REVOKED: "企业访问已撤销", ORGANIZATION_SUSPENDED: "企业访问已暂停", ORGANIZATION_CONTEXT_CHANGED: "当前企业已变化", ORGANIZATION_SELECTION_REQUIRED: "请选择当前企业", INVALID_UPSTREAM_RESPONSE: "资料响应无效" };
  return <ConsoleState kind="error" title={messages[code] ?? "资料读取失败"}><p>本次未取得资料。请确认登录状态或当前企业后重新读取。</p>{["AUTHENTICATION_REQUIRED", "IDENTITY_CONTEXT_CHANGED"].includes(code) ? <Button asChild variant="outline"><Link href={`/login?returnTo=${encodeURIComponent(`/workbench/account/${page}`)}`} prefetch={false}>重新登录</Link></Button> : null}</ConsoleState>;
}
