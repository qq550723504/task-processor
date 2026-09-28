import { notFound } from "next/navigation";
import { z } from "zod";
import { requireAccountUserId } from "@/lib/server/account-page-auth";
import { RecipientInvitation } from "@/components/workbench/account/invitations";
export default async function InvitationPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  if (!z.string().uuid().safeParse(id).success || id !== id.toLowerCase())
    notFound();
  const userId = await requireAccountUserId(`/invitations/${id}`);
  return (
    <RecipientInvitation key={`${userId}:${id}`} userId={userId} id={id} />
  );
}
