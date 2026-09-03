import { redirect } from "next/navigation";

export default async function QueuePage({
  params,
}: {
  params: Promise<{ taskId: string }>;
}) {
  const { taskId } = await params;

  redirect(`/listing-kits/${taskId}/workspace`);
}
