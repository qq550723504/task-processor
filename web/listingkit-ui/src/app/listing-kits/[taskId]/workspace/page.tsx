import { WorkspaceScreen } from "@/components/listingkit/workspace-screen";

export default async function WorkspacePage({
  params,
}: {
  params: Promise<{ taskId: string }>;
}) {
  const { taskId } = await params;

  return <WorkspaceScreen taskId={taskId} />;
}
