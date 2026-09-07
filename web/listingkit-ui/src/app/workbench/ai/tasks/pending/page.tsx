import { connection } from "next/server";
import { PendingTitleReviewPageContent } from "@/components/workbench/task-center/pending-title-review-page";
import { configuredProductReviewOrigin } from "@/lib/server/product-title-review-request";

export default async function PendingTitleReviewPage({ searchParams }: { searchParams: Promise<{ proposal_id?: string | string[] }> }) {
  await connection();
  const query = await searchParams;
  return <PendingTitleReviewPageContent available={!!configuredProductReviewOrigin()} initialProposalId={typeof query.proposal_id === "string" ? query.proposal_id : undefined} />;
}
