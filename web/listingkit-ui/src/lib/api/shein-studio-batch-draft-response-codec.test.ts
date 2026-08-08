import { describe, expect, it } from "vitest";

import { sheinStudioBatchDraftDetailContractFixture } from "@/lib/api/__fixtures__/shein-studio-batch-contract";
import { parseStudioBatchDraftDetailResponse } from "@/lib/api/shein-studio-batch-draft-response-codec";

describe("SHEIN Studio batch draft response codec", () => {
  it("parses the established detail contract", () => {
    expect(
      parseStudioBatchDraftDetailResponse(
        sheinStudioBatchDraftDetailContractFixture.response,
      ),
    ).toMatchObject(sheinStudioBatchDraftDetailContractFixture.response);
  });

  it("reports invalid detail fields as a 502 API shape error", () => {
    expect(() =>
      parseStudioBatchDraftDetailResponse({ batch: { id: 123 } }),
    ).toThrowError(
      expect.objectContaining({
        status: 502,
        payload: expect.objectContaining({
          issues: expect.arrayContaining([
            expect.objectContaining({ path: "batch.id" }),
          ]),
        }),
      }),
    );
  });
});
