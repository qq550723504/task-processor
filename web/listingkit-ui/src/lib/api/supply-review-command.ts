import { z } from "zod";
import { collectionID } from "../contracts/product-collection";
import { productTitleApplySchema,productTitleDecisionSchema,productTitleOrganizationSchema } from "./product-title-review";
import { applyProductTitleProposal,decideProductTitleProposal,ProductTitleReviewError } from "./product-title-review-client";
import type { SupplyIntent } from "./supply-chain";
import { SupplyAPIError } from "./supply-error";

const version=z.string().regex(/^[1-9][0-9]{0,18}$/).refine(v=>BigInt(v)<=BigInt("9223372036854775807"));
const binding={proposalId:collectionID,sourceId:collectionID,recordId:collectionID,productKey:productTitleOrganizationSchema,baseVersion:version};
export const supplyReviewDecisionCommandSchema=z.object({...binding,input:productTitleDecisionSchema}).strict();
export const supplyReviewApplyCommandSchema=z.object({...binding,input:productTitleApplySchema}).strict();
export async function supplyReviewCommand(intent:SupplyIntent,signal?:AbortSignal){
 const schema=intent.route==="review-decision"?supplyReviewDecisionCommandSchema:supplyReviewApplyCommandSchema;
 const input=schema.parse(intent.command);
 try{
  const request={organizationId:intent.organizationId,userId:intent.userId,proposalId:input.proposalId,idempotencyKey:collectionID.parse(intent.key),signal};
  const p=intent.route==="review-decision"?await decideProductTitleProposal({...request,input:productTitleDecisionSchema.parse(input.input)}):await applyProductTitleProposal({...request,input:productTitleApplySchema.parse(input.input)});
  let valid=p.proposal_id===input.proposalId&&p.owner===intent.userId&&p.input.product_key===input.productKey&&p.input.base_version===input.baseVersion;
  if(intent.route==="review-decision"){
   const decision=productTitleDecisionSchema.parse(input.input);
   valid=valid&&BigInt(p.revision)===BigInt(decision.expected_revision)+BigInt(1)&&p.state===(decision.action==="accept"?"accepted":decision.action==="reject"?"rejected":"pending")&&(decision.action!=="edit"||p.after===decision.title);
  }else valid=valid&&p.state==="applied"&&p.revision===input.input.expected_revision&&p.apply_receipt?.revision===input.input.expected_revision&&BigInt(p.apply_receipt.product_version)===BigInt(input.baseVersion)+BigInt(1);
  if(!valid)throw new SupplyAPIError("OUTCOME_UNKNOWN",503);
  return p;
 }catch(e){
  if(e instanceof SupplyAPIError)throw e;
  if(e instanceof ProductTitleReviewError)throw new SupplyAPIError(e.outcome==="unknown"?"OUTCOME_UNKNOWN":e.status===401?"AUTHENTICATION_REQUIRED":e.status===403?"PERMISSION_DENIED":e.status===409?"REVISION_CONFLICT":e.status>=500?"DEPENDENCY_UNAVAILABLE":"INVALID_REQUEST",e.status);
  throw new SupplyAPIError("OUTCOME_UNKNOWN",503);
 }
}
