import { expect, it } from "vitest";
import { agentResultSchema } from "./product-agent";

it("accepts backend-admitted candidate collections within the aggregate response limit", () => {
  const id = "11111111-1111-4111-8111-111111111111";
  const response = {
    runId:id, requestKey:id, operationId:id, productKey:"product", catalogVersion:"1", publicationId:"publication", targetPlatform:"shein",
    phase:"human_review_required", revision:"2", humanReviewRequired:true, canSubmitReview:true,
    candidate:{Changes:[{Field:"title",Value:"Valid title",EvidenceIDs:Array.from({length:40},()=>"source")}],Warnings:Array.from({length:40},()=>({Code:"warn",Field:"title",Message:"Preserved model warning",Metadata:null})),Rejections:[]},
    confidence:[], unresolved:["x".repeat(5000), ...Array.from({length:70},()=>"unresolved")], steps:[], tokens:50, estimatedCostMicros:30,currency:"CNY",usageStatus:"observed"
  };
  expect(new TextEncoder().encode(JSON.stringify(response)).length).toBeLessThan(64*1024);
  expect(agentResultSchema.safeParse(response).success).toBe(true);
});
