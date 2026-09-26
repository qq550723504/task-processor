import { redirect } from "next/navigation";
import { serverAuth } from "@/auth";
import { AccountShell } from "@/components/workbench/account/account-shell";
import { AuditPage } from "@/components/workbench/account/audit/audit-page";
import { readZitadelIdentityFromSession, readZitadelSessionError } from "@/lib/server/zitadel-auth";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";

const pathname = "/workbench/account/organization/audit";
export default async function AccountAuditPage() {
  const session = await serverAuth();
  const identity = readZitadelIdentityFromSession(session);
  if (!identity || !readZitadelServerAccessToken(session) || readZitadelSessionError(session)) redirect(`/login?returnTo=${encodeURIComponent(pathname)}`);
  return <AccountShell pathname={pathname} title="操作记录" description="查看当前企业的源账号操作，追溯已提交的变更。"><AuditPage expectedUserId={String(identity.userId)} /></AccountShell>;
}
