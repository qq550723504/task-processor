import { QueueScreen } from "@/components/listingkit/queue/queue-screen";

export default async function QueuePage({
  params,
}: {
  params: Promise<{ taskId: string }>;
}) {
  const { taskId } = await params;

  return <QueueScreen taskId={taskId} />;
}
