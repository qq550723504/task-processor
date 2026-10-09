import { notFound } from "next/navigation";
import { NotificationDetailPage } from "@/components/workbench/notifications/notification-center";
import { requireAccountUserId } from "@/lib/server/account-page-auth";
import { notificationRef as reference } from "@/lib/api/notifications";
import { isNotificationCenterAvailable } from "@/lib/server/notification-availability";
export default async function Page({ params }: { params: Promise<{ category: string; notificationRef: string }> }) {
  const { category, notificationRef } = await params;
  if (category !== "official" && category !== "business" || !reference.safeParse(notificationRef).success) notFound();
  const expectedUserId = await requireAccountUserId(`/workbench/notifications/${category}/${notificationRef}`);
  return isNotificationCenterAvailable() ? <NotificationDetailPage expectedUserId={expectedUserId} category={category} notificationRef={notificationRef} /> : <section className="p-8"><h1 className="text-2xl font-semibold">通知中心</h1><p className="mt-4 text-muted-foreground">当前环境尚未启用通知中心。</p></section>;
}
