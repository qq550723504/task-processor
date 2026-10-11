import {AIWorkbenchEntry} from "@/components/workbench/console/ai-workbench-entry";
import {isKnowledgeAvailable} from "@/lib/server/knowledge-availability";
import {isReportCenterAvailable} from "@/lib/server/report-center-availability";
import {configuredProductReviewOrigin} from "@/lib/server/product-title-review-request";
import {isSheinRecordsAvailable} from "@/lib/server/shein-records-availability";

export default function Page() {
  return <AIWorkbenchEntry knowledgeAvailable={isKnowledgeAvailable()} reportCenterAvailable={isReportCenterAvailable()} productReviewAvailable={configuredProductReviewOrigin() !== null} sheinRecordsAvailable={isSheinRecordsAvailable()} />;
}
