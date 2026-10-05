import { notFound } from "next/navigation";
import { BusinessTaskPage } from "@/components/workbench/ai-workbench/task-page";
import { isAcquisitionUUID } from "@/lib/contracts/product-acquisition";
export default async function Page({ params }: { params: Promise<{ taskId: string }> }) {
  const { taskId } = await params;
  if (!isAcquisitionUUID(taskId)) notFound();
  return <BusinessTaskPage taskId={taskId} />;
}
