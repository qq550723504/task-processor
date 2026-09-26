import { afterEach, expect, it, vi } from "vitest";
import { approveMainImage, readMainImage, readMainImageCandidates, startMainImage } from "./acquisition-main-image";

const operation = "11111111-1111-4111-8111-111111111111";
const run = "22222222-2222-4222-8222-222222222222";
const key = "33333333-3333-4333-8333-333333333333";
const scope = { userId: "actor", organizationId: "organization" };
afterEach(() => vi.restoreAllMocks());

it("starts one server-owned image request with the caller's retained key and source ID", async () => {
  const fetcher = vi.spyOn(globalThis, "fetch").mockResolvedValue(Response.json({ runId: run, status: "accepted" }, { status: 202 }));
  await expect(startMainImage(operation, "catalog-image-2", key, scope)).resolves.toEqual({ runId: run, status: "accepted" });
  const [url, init] = fetcher.mock.calls[0];
  expect(url).toBe(`/api/workbench/sourcing/1688/acquisitions/${operation}/main-image`);
  expect((init as RequestInit).body).toBe('{"sourceImageId":"catalog-image-2"}');
  expect(new Headers((init as RequestInit).headers).get("Idempotency-Key")).toBe(key);
  expect(new Headers((init as RequestInit).headers).get("X-Expected-Organization-ID")).toBe("organization");
});

it("reads exact candidates and run; approval uses the same durable action ID", async () => {
  const fetcher = vi.spyOn(globalThis, "fetch")
    .mockResolvedValueOnce(Response.json({ operationId: operation, candidates: [{ id: "catalog-image-2", displayUrl: "https://example.com/source.png" }] }))
    .mockResolvedValueOnce(Response.json({ runId: run, status: "awaiting_final_approval", planRevision: 1, resultDigest: "digest", approvalAvailable: true, imageUrl: "https://example.com/output.png" }))
    .mockResolvedValueOnce(Response.json({ runId: run, status: "accepted" }, { status: 202 }));
  expect((await readMainImageCandidates(operation, scope)).candidates[0].id).toBe("catalog-image-2");
  expect((await readMainImage(operation, run, scope)).approvalAvailable).toBe(true);
  await approveMainImage(operation, run, { planRevision: 1, resultDigest: "digest", actionId: key }, scope);
  const init = fetcher.mock.calls[2][1] as RequestInit;
  expect(init.body).toBe(JSON.stringify({ planRevision: 1, resultDigest: "digest", actionId: key }));
});

it("treats an invalid dispatched Start response as unknown rather than allocating a new key", async () => {
  vi.spyOn(globalThis, "fetch").mockResolvedValue(Response.json({ runId: "not-a-run", status: "accepted" }, { status: 202 }));
  await expect(startMainImage(operation, "catalog-image-2", key, scope)).rejects.toMatchObject({ code: "OUTCOME_UNKNOWN" });
});

it.each([
  ["AUTHENTICATION_REQUIRED", 401],
  ["ORGANIZATION_ACCESS_REVOKED", 403],
  ["ORGANIZATION_ACCESS_DENIED", 403],
  ["DEADLINE_EXCEEDED", 504],
] as const)("preserves a confirmed %s rejection without retry", async (code, status) => {
  const fetcher = vi.spyOn(globalThis, "fetch").mockResolvedValue(Response.json({ code }, { status }));
  await expect(startMainImage(operation, "catalog-image-2", key, scope)).rejects.toMatchObject({ code, status });
  expect(fetcher).toHaveBeenCalledOnce();
});
