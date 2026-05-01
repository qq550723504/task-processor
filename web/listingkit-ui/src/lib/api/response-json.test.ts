import { describe, expect, it } from "vitest";

import {
  parseJsonResponse,
  ResponseJsonParseError,
} from "@/lib/api/response-json";

describe("parseJsonResponse", () => {
  it("returns undefined for empty body", async () => {
    const response = new Response(null, { status: 202 });

    await expect(parseJsonResponse(response)).resolves.toBeUndefined();
  });

  it("parses valid json body", async () => {
    const response = new Response(JSON.stringify({ ok: true }), { status: 200 });

    await expect(parseJsonResponse<{ ok: boolean }>(response)).resolves.toEqual({
      ok: true,
    });
  });

  it("throws a typed error for truncated json", async () => {
    const response = new Response("{", { status: 200 });

    await expect(parseJsonResponse(response)).rejects.toBeInstanceOf(
      ResponseJsonParseError,
    );
  });
});
