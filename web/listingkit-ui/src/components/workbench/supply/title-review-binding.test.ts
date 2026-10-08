import {expect,it} from "vitest";
import {titleProposalFixture} from "@/test/product-title-review-fixture";
import {validateSupplyTitleReview,reviewedSupplyDraft} from "./title-review-binding";
import type {SupplyTarget} from "@/lib/contracts/supply-chain";
const p=titleProposalFixture();
const r={id:"record",revision:1,source:{id:"source",source:{productKey:p.input.product_key}},effectiveVersion:p.input.base_version,merchant:{store_id:"store"},input:{draft:{product:{multi_language_name_list:[{language:"en",name:p.before}],multi_language_desc_list:[{language:"en",name:"manual description"}]},images:[]}}} as unknown as SupplyTarget;
it("binds the real proposal to the pinned immutable target and owner",()=>{
 const item={sourceId:"source",recordId:"record",recordRevision:1,status:"review" as const,resultReference:p.proposal_id};
 expect(()=>validateSupplyTitleReview(p,r,item,p.owner,"store")).not.toThrow();
 for(const proposal of [{...p,owner:"other"},{...p,input:{...p.input,base_version:"99"}},{...p,proposal_id:"other"}])expect(()=>validateSupplyTitleReview(proposal,r,item,p.owner,"store")).toThrow();
});
it("uses only a real applied title receipt and preserves manual description",()=>{
 const applied={...p,state:"applied" as const,after:"Reviewed title",apply_receipt:{proposal_id:p.proposal_id,product_version:String(BigInt(p.input.base_version)+BigInt(1)),publication_id:"real-publication",revision:p.revision,actor:p.owner,at:"2026-10-09T00:00:00Z"}};
 expect(reviewedSupplyDraft(r,applied).product.multi_language_name_list).toEqual([{language:"en",name:"Reviewed title"}]);
 expect(reviewedSupplyDraft(r,applied).product.multi_language_desc_list).toEqual(r.input.draft.product.multi_language_desc_list);
 expect(()=>reviewedSupplyDraft(r,p)).toThrow();
});
