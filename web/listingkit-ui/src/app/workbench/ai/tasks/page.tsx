import { BusinessTaskPage } from "@/components/workbench/ai-workbench/task-page";
import { configuredProductReviewOrigin } from "@/lib/server/product-title-review-request";
import { isSheinRecordsAvailable } from "@/lib/server/shein-records-availability";
import { connection } from "next/server";

export default async function Page() {
  await connection();
  return <BusinessTaskPage productReviewAvailable={configuredProductReviewOrigin() !== null} sheinRecordsAvailable={isSheinRecordsAvailable()} />;
}
