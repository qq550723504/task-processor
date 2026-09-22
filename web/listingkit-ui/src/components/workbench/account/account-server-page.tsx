import { requireAccountUserId } from "@/lib/server/account-page-auth";
import { AccountPage, accountPagePath, type AccountPageKind } from "./account-page";

export async function AccountServerPage({ page }: { page: AccountPageKind }) {
  const expectedUserId = await requireAccountUserId(accountPagePath(page));
  return <AccountPage page={page} expectedUserId={expectedUserId} />;
}
