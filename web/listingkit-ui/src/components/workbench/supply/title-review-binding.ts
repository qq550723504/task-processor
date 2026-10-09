import type {ProductTitleProposal} from "@/lib/api/product-title-review";
import type {SupplyTarget,SupplyOperationItem} from "@/lib/contracts/supply-chain";
export function validateSupplyTitleReview(p:ProductTitleProposal,r:SupplyTarget,item:SupplyOperationItem,userId:string,storeId:string){
 if(p.proposal_id!==item.resultReference||p.owner!==userId||r.id!==item.recordId||r.revision!==item.recordRevision||r.source.id!==item.sourceId||r.merchant.store_id!==storeId||p.input.product_key!==r.source.source.productKey||p.input.base_version!==r.effectiveVersion)throw new Error("REVIEW_BINDING_CONFLICT");
}
export function reviewedSupplyDraft(r:SupplyTarget,p:ProductTitleProposal){
 if(p.state!=="applied"||!p.apply_receipt||p.apply_receipt.proposal_id!==p.proposal_id||p.apply_receipt.revision!==p.revision||p.input.product_key!==r.source.source.productKey||p.input.base_version!==r.effectiveVersion||BigInt(p.apply_receipt.product_version)!==BigInt(r.effectiveVersion)+BigInt(1))throw new Error("REVIEW_BINDING_CONFLICT");
 const draft=structuredClone(r.input.draft);
 const title=draft.product.multi_language_name_list.find(n=>n.language==="en");
 if(title)title.name=p.after;else draft.product.multi_language_name_list.push({language:"en",name:p.after});
 return draft;
}
