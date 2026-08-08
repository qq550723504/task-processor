import { describe, expect, it } from "vitest";

import { sheinStudioBatchUpsertContractFixture } from "@/lib/api/__fixtures__/shein-studio-batch-contract";
import { buildStudioBatchDraftUpsertPayload } from "@/lib/api/shein-studio-batch-draft-request-codec";

describe("SHEIN Studio batch draft request codec", () => {
  it("encodes the established upsert wire contract", () => {
    const wirePayload = JSON.parse(
      JSON.stringify(
        buildStudioBatchDraftUpsertPayload(sheinStudioBatchUpsertContractFixture.input),
      ),
    );

    expect(wirePayload).toEqual(sheinStudioBatchUpsertContractFixture.expectedBody);
  });

  it("does not serialize a synthesized name or empty legacy snapshot for an update", () => {
    const payload = buildStudioBatchDraftUpsertPayload({
      ...sheinStudioBatchUpsertContractFixture.input,
      id: "batch-1",
      name: " ",
      legacyCompatibilitySnapshot: undefined,
    });

    expect(payload.batch_name).toBeUndefined();
    expect(payload.legacy_compatibility_snapshot).toBeUndefined();
    const wirePayload = JSON.parse(JSON.stringify(payload));
    expect(wirePayload).not.toHaveProperty("batch_name");
    expect(wirePayload).not.toHaveProperty("legacy_compatibility_snapshot");
  });

  it("normalizes serialized hot-style reference fields", () => {
    const payload = buildStudioBatchDraftUpsertPayload({
      ...sheinStudioBatchUpsertContractFixture.input,
      hotStyleReferenceImageUrls: [" https://example.com/hot.png "],
      hotStyleReferenceBrief: "brief",
      hotStyleReferencePrompt: "prompt",
    });

    expect(payload).toMatchObject({
      hot_style_reference_image_urls: ["https://example.com/hot.png"],
      hot_style_reference_brief: "brief",
      hot_style_reference_prompt: "prompt",
    });
  });
});
