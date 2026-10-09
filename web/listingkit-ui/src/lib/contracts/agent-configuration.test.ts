import { expect, it } from "vitest";
import { agentStartRequestSchema } from "./product-agent";
import { catalogEntrySchema, configReceiptSchema } from "./agent-configuration";
const template = {
  templateId: "8fc227bb-b572-4138-8e2a-5f1a0be98617",
  revision: "9223372036854775807",
};
it("retains the exact template revision and explicit no-Knowledge choice", () => {
  const parsed = agentStartRequestSchema.parse({
    targetPlatform: "shein",
    templateSelection: template,
  });
  expect(parsed).toEqual({
    targetPlatform: "shein",
    templateSelection: template,
  });
  expect(parsed.knowledgeSelection).toBeUndefined();
});

it("accepts the image agent catalog and receipt without treating its parameters as title configuration",()=>{
  const entry={agent:{agentId:"product.image.agent",activation:"NOT_ENABLED",revision:"",activationEpoch:"",defaultTemplate:null,updatedAt:"2026-10-09T00:00:00Z"},name:"商品图片智能体",description:"整套生成与人工选择",definitionVersion:"v1.0.0",parameterSchema:"image-config-v1",canConfigure:true,canUse:true,canReadRuns:false,capabilities:[]};
  expect(catalogEntrySchema.safeParse(entry).success).toBe(true);
  expect(catalogEntrySchema.safeParse({...entry,parameterSchema:"title-config-v1"}).success).toBe(false);
  expect(configReceiptSchema.safeParse({commandId:template.templateId,operation:"enable",agentId:"product.image.agent",revision:"1",noop:false,committedAt:"2026-10-09T00:00:00Z"}).success).toBe(true);
});
it("rejects null, partial and rounded template references", () => {
  for (const value of [
    null,
    {},
    { ...template, revision: 3 },
    { ...template, revision: "03" },
    { ...template, revision: "invalid" },
    { ...template, revision: "9223372036854775808" },
  ])
    expect(
      agentStartRequestSchema.safeParse({
        targetPlatform: "shein",
        templateSelection: value,
      }).success,
    ).toBe(false);
});
