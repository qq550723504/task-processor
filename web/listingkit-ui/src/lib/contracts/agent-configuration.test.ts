import { expect, it } from "vitest";
import { agentStartRequestSchema } from "./product-agent";
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
