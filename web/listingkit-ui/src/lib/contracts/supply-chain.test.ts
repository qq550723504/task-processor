import { expect,it } from "vitest";
import { officialDraftInputSchema,supplyPath } from "./supply-chain";
import { initialSupplyDraft } from "@/components/workbench/supply/draft-fields";
import type { SupplySourceDetail } from "./supply-chain";
it("allows one approved image to be assigned to every real SKU in a large batch",()=>{
 const draft=initialSupplyDraft({product:{}} as SupplySourceDetail);
 draft.images=Array.from({length:205},(_,sku)=>({group:"sku" as const,skc:0,sku,asset_id:"approved-original",sort:1,type:1}));
 expect(officialDraftInputSchema.safeParse(draft).success).toBe(true);
});
it("admits private retained preparation and operation history reads",()=>{
 const id="11111111-1111-4111-8111-111111111111";
 expect(supplyPath("GET",["supply-preparations",id])).toBe("preparation");expect(supplyPath("GET",["supply-preparations",id,"operations"])).toBe("operations");
});
