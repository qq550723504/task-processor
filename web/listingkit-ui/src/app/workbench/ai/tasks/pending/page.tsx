import { BusinessTaskPage } from "@/components/workbench/ai-workbench/task-page";
import { configuredProductReviewOrigin } from "@/lib/server/product-title-review-request";
import { connection } from "next/server";

export default async function Page() {
  await connection();
  return <BusinessTaskPage mode="pending" productReviewAvailable={configuredProductReviewOrigin() !== null} />;
}
