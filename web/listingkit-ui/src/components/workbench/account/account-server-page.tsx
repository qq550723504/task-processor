import { requireAccountUserId } from "@/lib/server/account-page-auth";
import { AccountPage } from "./account-page";
import { accountPagePath, type AccountPageKind } from "./account-page-route";

export async function AccountServerPage({ page }: { page: AccountPageKind }) {
  const expectedUserId = await requireAccountUserId(accountPagePath(page));
  return <AccountPage page={page} expectedUserId={expectedUserId} />;
}
