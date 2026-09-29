import { notFound } from "next/navigation";
import { AgentPage } from "@/components/workbench/agents/agent-page";
export default async function Page({
  params,
}: {
  params: Promise<{ agentId: string }>;
}) {
  const { agentId } = await params;
  if (agentId !== "product.title.agent") notFound();
  return <AgentPage mode="mine" id={agentId} />;
}
