import { z } from "zod";
import { isAcquisitionUUID } from "./product-acquisition";
const id=z.string().refine(isAcquisitionUUID);
export const knowledgeSelectionSchema=z.strictObject({knowledgeBaseId:id});
export const productKnowledgeSchema=z.strictObject({
 status:z.enum(["available","unavailable","uncited"]),originAgentRunId:id,
 citations:z.array(z.strictObject({id,sourceId:id,revisionId:id,name:z.string().max(1024),location:z.string().max(512),excerpt:z.string().max(640),state:z.enum(["AVAILABLE","PARTIAL"])})).max(64),
}).refine(v=>v.status==="available" ? v.citations.length>0 && new Set(v.citations.map(c=>c.id)).size===v.citations.length : v.citations.length===0);
export type ProductKnowledge=z.infer<typeof productKnowledgeSchema>;
