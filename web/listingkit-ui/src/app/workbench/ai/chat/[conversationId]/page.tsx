import { notFound } from "next/navigation";
import { ChatPage } from "@/components/workbench/ai-workbench/chat-page";
import { isAcquisitionUUID } from "@/lib/contracts/product-acquisition";
export default async function Page({ params }: { params: Promise<{ conversationId: string }> }) {
  const { conversationId } = await params;
  if (!isAcquisitionUUID(conversationId)) notFound();
  return <ChatPage mode="detail" conversationId={conversationId} />;
}
