"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useState } from "react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { AccountReadError, getAccountBusinessProfile, getAccountOrganization, getAccountProfile } from "@/lib/api/account";
import { ConsoleState } from "../console/console-page";
import { AccountShell } from "./account-shell";
import { AccountOverviewView, OrganizationView, ProfileView } from "./account-views";
import styles from "./account.module.css";
import { accountPagePath, type AccountPageKind } from "./account-page-route";

export type { AccountPageKind } from "./account-page-route";
type ProfileSection = "summary" | "settings" | "business" | "verification";

function profileSection(page: AccountPageKind): ProfileSection | undefined {
  return page === "profile" ? "summary" : page === "profile-settings" ? "settings" : page === "profile-business" ? "business" : page === "profile-verification" ? "verification" : undefined;
}

export function AccountPage({ page, expectedUserId }: { page: AccountPageKind; expectedUserId: string }) {
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
  const title = page === "overview" ? "我的账户" : page === "profile" ? "账户资料" : page === "profile-settings" ? "账户设置" : page === "profile-business" ? "经营画像" : page === "profile-verification" ? "认证信息" : "企业空间";
  const authError = [context.error, context.blockingError].some(error => error?.code === "AUTHENTICATION_REQUIRED");
  const identityChanged = !!context.user && context.user.id !== expectedUserId;
  const organization = context.effectiveOrganization;
  // A keyed request subtree discards both visible data and consumed query signals on context changes.
  const scope = JSON.stringify([expectedUserId, context.user?.id, organization?.id, context.roles, context.error?.code, context.blockingError?.code, context.selectionRequired, context.isLoading, context.isSwitching]);
  let content;
  if (leaving || authError || identityChanged) content = <ReadError page={page} code={identityChanged ? "IDENTITY_CONTEXT_CHANGED" : "AUTHENTICATION_REQUIRED"} />;
  else if ((context.isSwitching && page !== "profile-verification") || ((page === "organization" || page === "overview" || page === "profile-business") && context.isLoading)) content = <ConsoleState kind="loading" title="正在确认当前上下文">旧资料已清除。</ConsoleState>;
  else if ((page === "organization" || page === "overview" || page === "profile-business") && (context.error || context.blockingError || !context.user || !organization || context.selectionRequired)) content = <ReadError code={context.blockingError?.code ?? context.error?.code ?? "ORGANIZATION_SELECTION_REQUIRED"} page={page} />;
  else content = <ScopedAccount key={`${page}:${scope}`} page={page} scope={scope} expectedUserId={expectedUserId} organizationId={page === "profile-verification" && (context.isSwitching || context.isLoading || context.selectionRequired || context.error || context.blockingError) ? undefined : organization?.id} />;
  return <AccountShell pathname={accountPagePath(page)} title={title} description={page === "overview" ? "查看当前账户、企业与收益状态" : page === "profile" ? "查看你的账户信息与当前资料状态" : page === "profile-settings" ? "管理账户信息、联系方式与登录安全" : page === "profile-business" ? "维护用于业务服务匹配的经营画像" : page === "profile-verification" ? "查看身份与企业授权认证状态" : "查看当前企业信息与项目访问权限"}>{content}</AccountShell>;
}

function ScopedAccount({ page, scope, expectedUserId, organizationId }: { page: AccountPageKind; scope: string; expectedUserId: string; organizationId?: string }) {
  const [sequence, setSequence] = useState(0);
  const [identityVerificationOutcomeUnknown, setIdentityVerificationOutcomeUnknown] = useState(false);
  const [identityContactOutcomeUnknown, setIdentityContactOutcomeUnknown] = useState(false);
  const clearUnknownOutcomes = () => { setIdentityVerificationOutcomeUnknown(false); setIdentityContactOutcomeUnknown(false); setSequence(value => value + 1); };
  return <><AccountRequest key={sequence} page={page} scope={scope} expectedUserId={expectedUserId} organizationId={organizationId} sequence={sequence} identityVerificationOutcomeUnknown={identityVerificationOutcomeUnknown} onIdentityVerificationOutcomeUnknown={() => setIdentityVerificationOutcomeUnknown(true)} identityContactOutcomeUnknown={identityContactOutcomeUnknown} onIdentityContactOutcomeUnknown={() => setIdentityContactOutcomeUnknown(true)} /><div className={styles.refresh}><Button variant="outline" onClick={clearUnknownOutcomes}>刷新资料</Button></div></>;
}
function AccountRequest({ page, scope, expectedUserId, organizationId, sequence, identityVerificationOutcomeUnknown, onIdentityVerificationOutcomeUnknown, identityContactOutcomeUnknown, onIdentityContactOutcomeUnknown }: { page: AccountPageKind; scope: string; expectedUserId: string; organizationId?: string; sequence: number; identityVerificationOutcomeUnknown: boolean; onIdentityVerificationOutcomeUnknown: () => void; identityContactOutcomeUnknown: boolean; onIdentityContactOutcomeUnknown: () => void }) {
  const response = useQuery({ queryKey: ["account", page, scope, sequence], queryFn: async ({ signal }) => {
    const business = async (required = false) => {
      if (!organizationId) { if (required) throw new AccountReadError(409, "ORGANIZATION_SELECTION_REQUIRED"); return null; }
      if (required) return getAccountBusinessProfile({ expectedUserId, expectedOrganizationId: organizationId, signal });
      return getAccountBusinessProfile({ expectedUserId, expectedOrganizationId: organizationId, signal }).catch(() => null);
    };
    if (profileSection(page)) {
      const readsBusinessProfile = page === "profile" || page === "profile-business";
      const readsOrganization = page === "profile-verification" && organizationId;
      const [profile, businessProfile, organization] = await Promise.all([
        getAccountProfile({ expectedUserId, signal }),
        readsBusinessProfile ? business(page === "profile-business") : Promise.resolve(null),
        readsOrganization ? getAccountOrganization({ expectedUserId, expectedOrganizationId: organizationId!, signal }).catch(error => {
          if (error instanceof AccountReadError && ["AUTHENTICATION_REQUIRED", "IDENTITY_CONTEXT_CHANGED"].includes(error.code)) throw error;
          return null;
        }) : Promise.resolve(null),
      ]);
      return { kind: "profile" as const, profile, business: businessProfile, organization };
    }
    if (page === "overview") { const [profile, businessProfile, organization] = await Promise.all([getAccountProfile({ expectedUserId, signal }), business(), getAccountOrganization({ expectedUserId, expectedOrganizationId: organizationId!, signal })]); return { kind: "overview" as const, profile, business: businessProfile, organization }; }
    return { kind: "organization" as const, organization: await getAccountOrganization({ expectedUserId, expectedOrganizationId: organizationId!, signal }) };
  }, gcTime: 0, staleTime: 0, retry: false, refetchOnWindowFocus: true, refetchOnReconnect: true });
  if (response.isPending || response.isFetching) return <ConsoleState kind="loading" title="正在读取资料">正在确认当前身份和访问权限。</ConsoleState>;
  if (response.isError) return <ReadError code={response.error instanceof AccountReadError ? response.error.code : "UNKNOWN"} page={page} />;
  if (response.data.kind === "organization") return <OrganizationView data={response.data.organization} />;
  if (response.data.kind === "overview") return <AccountOverviewView profile={response.data.profile} business={response.data.business} organization={response.data.organization} />;
  return <ProfileView data={response.data.profile} business={response.data.business} organization={response.data.organization} organizationId={organizationId} section={profileSection(page)} identityVerificationOutcomeUnknown={identityVerificationOutcomeUnknown} onIdentityVerificationOutcomeUnknown={onIdentityVerificationOutcomeUnknown} identityContactOutcomeUnknown={identityContactOutcomeUnknown} onIdentityContactOutcomeUnknown={onIdentityContactOutcomeUnknown} />;
}
function ReadError({ code, page = "profile" }: { code: string; page?: AccountPageKind }) {
  const messages: Record<string, string> = { AUTHENTICATION_REQUIRED: "登录已失效", IDENTITY_CONTEXT_CHANGED: "登录身份已变化", ACCOUNT_NOT_CONFIGURED: "账户资料服务尚未配置", DEPENDENCY_UNAVAILABLE: "资料服务暂不可用", DEADLINE_EXCEEDED: "资料读取超时", PERMISSION_DENIED: "无查看权限", ORGANIZATION_ACCESS_DENIED: "企业访问被拒绝", ORGANIZATION_ACCESS_REVOKED: "企业访问已撤销", ORGANIZATION_SUSPENDED: "企业访问已暂停", ORGANIZATION_CONTEXT_CHANGED: "当前企业已变化", ORGANIZATION_SELECTION_REQUIRED: "请选择当前企业", INVALID_UPSTREAM_RESPONSE: "资料响应无效" };
  const returnTo = accountPagePath(page);
  return <ConsoleState kind="error" title={messages[code] ?? "资料读取失败"}><p>本次未取得资料。请确认登录状态或当前企业后重新读取。</p>{["AUTHENTICATION_REQUIRED", "IDENTITY_CONTEXT_CHANGED"].includes(code) ? <Button asChild variant="outline"><Link href={`/login?returnTo=${encodeURIComponent(returnTo)}`} prefetch={false}>重新登录</Link></Button> : null}</ConsoleState>;
}
