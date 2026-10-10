import { notFound } from "next/navigation";
import { PrivateAgentPage } from "@/components/workbench/agents/private-agent-page";
import { customId } from "@/lib/api/agent-customization";
export default async function Page({ params }: { params: Promise<{ deliveryId: string ;}> ;}) {
  const { deliveryId } = await params;
  if (!customId.safeParse(deliveryId).success) notFound();
  return <PrivateAgentPage id={deliveryId} offlineTrial={process.env.LISTINGKIT_PRIVATE_DRAFT_TRIAL_ENABLED === "true"} />;
}
