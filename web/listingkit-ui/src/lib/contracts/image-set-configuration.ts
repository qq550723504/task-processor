import { z } from "zod";
import { configVersion,templateInputSchema } from "./agent-configuration";
import { knowledgeId } from "../api/knowledge";

export const carouselTasks = [
  {purpose:"product_identity",label:"商品识别主图",description:"第一眼看懂商品，主体清晰"},
  {purpose:"purchase_reason",label:"核心购买理由图",description:"突出最值得点击的核心价值"},
  {purpose:"need_solution",label:"核心需求解决图",description:"直接回应买家最高优先级需求"},
  {purpose:"structure",label:"产品结构确认图",description:"展示关键角度、结构与完整形态"},
  {purpose:"detail_evidence",label:"核心细节证据图",description:"材质、工艺、印花等真实细节"},
  {purpose:"usage_scene",label:"核心使用场景图",description:"用真实场景建立使用想象"},
  {purpose:"choice_reason",label:"卖点选择理由图",description:"压缩核心优势，强化差异"},
  {purpose:"purchase_specs",label:"规格购买确认图",description:"颜色、尺码、规格等购买信息",evidence:"specifications"},
] as const;
export const detailTasks = [
  {purpose:"product_overview",label:"商品全貌",description:"清晰展示商品完整外观"},
  {purpose:"key_benefits",label:"核心卖点",description:"以真实商品事实说明优势"},
  {purpose:"use_scenario",label:"使用场景",description:"呈现符合商品用途的场景"},
  {purpose:"structure_functions",label:"结构功能",description:"展示结构及已确认功能"},
  {purpose:"detail_closeup",label:"细节特写",description:"保留材质与工艺真实细节"},
  {purpose:"specification_dimensions",label:"规格尺寸",description:"使用已确认的尺寸与规格",evidence:"specifications"},
  {purpose:"usage_steps",label:"使用步骤",description:"按已提供的说明呈现步骤",evidence:"instructions"},
  {purpose:"packaging_accessories",label:"包装配件",description:"仅展示真实包装和随附配件",evidence:"accessories"},
] as const;
const text = (maximumBytes:number) => z.string().refine(value=>new TextEncoder().encode(value).length<=maximumBytes&&!/[\u0000-\u0008\u000b-\u001f\u007f-\u009f]/.test(value));
const imageContentTaskSchema=z.strictObject({id:z.string().regex(/^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$/),purpose:z.string().min(1).max(64),brief:text(2048).optional()});
export const imageSetTemplateSchema = z.strictObject({
  schema:z.literal("image-config-v1"),mode:z.enum(["standard","custom"]),shareOriginals:z.boolean(),
  background:text(1024).refine(value=>value.trim().length>0),language:z.enum(["en","zh","es","fr","de","ja"]),
  carousel:z.array(imageContentTaskSchema).max(32),detail:z.array(imageContentTaskSchema).max(32),
}).superRefine((template,ctx)=>{
  const count=template.carousel.length+template.detail.length;
  if(count<1||count>32||new TextEncoder().encode(JSON.stringify(template)).length>64*1024)ctx.addIssue({code:"custom",message:"选择 1–32 个任务，并缩短过长的配置。"});
  const ids=new Set<string>();
  for(const [group,definitions] of [[template.carousel,carouselTasks],[template.detail,detailTasks]] as const){
    const purposes=new Set<string>();
    for(const task of group){
      if(ids.has(task.id))ctx.addIssue({code:"custom",message:"任务编号不能重复。"});ids.add(task.id);
      if(template.mode==="custom"){
        if(task.purpose!=="custom"||!task.brief?.trim())ctx.addIssue({code:"custom",message:"请填写每个自定义任务的指令。"});
      }else if(!definitions.some(definition=>definition.purpose===task.purpose)||purposes.has(task.purpose))ctx.addIssue({code:"custom",message:"请选择当前内容任务，且每组不能重复。"});
      purposes.add(task.purpose);
    }
  }
});
export type ImageSetTemplate=z.infer<typeof imageSetTemplateSchema>;
export const imageTemplateSchema=z.strictObject({
  templateId:knowledgeId,agentId:z.literal("product.image.agent"),lifecycle:z.enum(["ACTIVE","ARCHIVED"]),
  revision:configVersion,version:configVersion,schemaVersion:z.literal("image-config-v1"),name:z.string().min(1).max(512),
  targetPlatform:z.enum(["product","shein","temu","amazon"]),image:imageSetTemplateSchema,createdAt:z.string().datetime({offset:true}),
});
export const imageTemplatesPageSchema=z.strictObject({items:z.array(imageTemplateSchema).max(100),nextCursor:z.string().max(180)});
export type ImageAgentTemplate=z.infer<typeof imageTemplateSchema>;

export const imageTemplateInputSchema=z.strictObject({name:templateInputSchema.shape.name,targetPlatform:z.enum(["product","shein","temu","amazon"]),image:imageSetTemplateSchema});
