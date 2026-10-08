import { NotificationCenterPage } from "@/components/workbench/notifications/notification-center";
import { requireAccountUserId } from "@/lib/server/account-page-auth";
import { isNotificationCenterAvailable } from "@/lib/server/notification-availability";
export default async function Page() {
  const expectedUserId = await requireAccountUserId("/workbench/notifications");
  return isNotificationCenterAvailable() ? <NotificationCenterPage expectedUserId={expectedUserId} /> : <section className="p-8"><h1 className="text-2xl font-semibold">通知中心</h1><p className="mt-4 text-muted-foreground">当前环境尚未启用通知中心。</p></section>;
}
