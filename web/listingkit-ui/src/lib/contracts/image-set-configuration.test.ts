import { expect, it } from "vitest";
import { imageSetTemplateSchema } from "./image-set-configuration";

const template = { schema:"image-config-v1",mode:"standard",shareOriginals:true,background:"白色背景",language:"zh",carousel:[{id:"identity",purpose:"product_identity"}],detail:[{id:"overview",purpose:"product_overview"},{id:"steps",purpose:"usage_steps"}] };

it("keeps independent carousel and detail tasks while sharing only originals",()=>{
  expect(imageSetTemplateSchema.parse(template)).toEqual(template);
});
it("bounds custom instructions and rejects invalid or duplicate content intentions",()=>{
  const custom={...template,mode:"custom",detail:[],carousel:Array.from({length:32},(_,index)=>({id:`custom-${index}`,purpose:"custom",brief:"真实商品特写"}))};
  expect(imageSetTemplateSchema.safeParse(custom).success).toBe(true);
  for(const bad of [
    {...custom,carousel:[...custom.carousel,{id:"custom-32",purpose:"custom",brief:"其他"}]},
    {...custom,carousel:[{id:"a",purpose:"custom",brief:""}]},
    {...template,background:"中".repeat(342)},
    {...template,detail:[{id:"steps",purpose:"packaging_accessories"},{id:"duplicate",purpose:"packaging_accessories"}]},
    {...template,detail:[{id:"unknown",purpose:"made_up"}]},
    {...template,detail:[{id:"identity",purpose:"product_overview"}]},
  ]) expect(imageSetTemplateSchema.safeParse(bad).success).toBe(false);
});
