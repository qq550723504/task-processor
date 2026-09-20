import { redirect } from "next/navigation";
import { serverAuth } from "@/auth";
import { MembersPage } from "@/components/workbench/account/members-page";
import { readZitadelIdentityFromSession, readZitadelSessionError } from "@/lib/server/zitadel-auth";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";

export default async function Page() {
  const session = await serverAuth(); const identity = readZitadelIdentityFromSession(session);
  if (!identity || !readZitadelServerAccessToken(session) || readZitadelSessionError(session)) redirect(`/login?returnTo=${encodeURIComponent("/workbench/account/organization/members")}`);
  return <MembersPage expectedUserId={String(identity.userId)} />;
}
