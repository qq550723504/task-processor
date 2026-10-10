import {expect,it} from "vitest";
import {collectionItemsSchema,collectionBatchesSchema} from "./product-collection";
import {sourceDetailSchema} from "./supply-chain";
const id="11111111-1111-4111-8111-111111111111";
it("keeps retained market and SDS sources usable in My Data and the existing Supply Chain",()=>{
 for(const kind of ["market","sds_template","sds_finished"]){
  const item={id,batchId:id,revision:1,createdAt:"2026-10-10T00:00:00Z",source:{productKey:"received-product",publicationId:id,version:"1",kind,operationId:id}};
  expect(collectionItemsSchema.safeParse({items:[item],total:1}).success).toBe(true);
  expect(collectionBatchesSchema.safeParse({items:[{id,name:"真实来源",kind,revision:2,count:1,createdAt:item.createdAt}],total:1}).success).toBe(true);
  expect(sourceDetailSchema.safeParse({source:{id,preparationId:id,collectionItemId:id,collectionRevision:1,source:item.source},product:{title:"来源商品"},images:[]}).success).toBe(true);
 }
});
