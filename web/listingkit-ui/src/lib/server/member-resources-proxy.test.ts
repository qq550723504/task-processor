import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { proxyMemberResources } from "./member-resources-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";

const position = {
  free: "2",
  reserved: "0",
  consumed: "0",
  version: "1",
  recorded: true,
};
const receipt = {
  organizationId: "org-1",
  memberId: "member-1",
  resourceType: "store_renewal_period",
  operationId: "original-key",
  position,
  unallocated: "3",
  allocated: "2",
  grossCredit: "0",
  debtRepaid: "0",
  netCredit: "0",
  replayed: false,
};
function request(path = "/members/member-1/transfers", method = "POST") {
  return new Request(`http://localhost/api/account/member-resources${path}`, {
    method,
    headers: {
      cookie: `${WORKBENCH_COOKIE_NAME}=org-1`,
      Origin: "http://localhost",
      "X-Expected-User-ID": "actor-1",
      "X-Expected-Organization-ID": "org-1",
      "Content-Type": "application/json",
      "Idempotency-Key": "original-key",
    },
    ...(method === "GET"
      ? {}
      : {
          body: JSON.stringify({
            resourceType: "store_renewal_period",
            action: "allocate",
            quantity: "2",
            expectedVersion: "0",
          }),
        }),
  });
}
beforeEach(() => {
  vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://backend.example/api/v1");
  vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://localhost");
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});
it("forwards only the bound member operation and validates its immutable receipt", async () => {
  const fetch = vi.fn().mockResolvedValue(Response.json(receipt));
  vi.stubGlobal("fetch", fetch);
  expect(
    (await proxyMemberResources(request(), "token", "actor-1")).status,
  ).toBe(200);
  expect(fetch.mock.calls[0][0]).toBe(
    "http://backend.example/api/v1/account/organization/resources/member-resources/members/member-1/transfers",
  );
  expect(
    (fetch.mock.calls[0][1].headers as Headers).get("Idempotency-Key"),
  ).toBe("original-key");
});
it.each(["wrong-user", "wrong-org", "cross-origin", "arbitrary-path"])(
  "rejects %s before forwarding",
  async (mode) => {
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    const req = request(mode === "arbitrary-path" ? "/../profile" : undefined);
    if (mode === "wrong-user") req.headers.set("X-Expected-User-ID", "someone");
    if (mode === "wrong-org")
      req.headers.set("X-Expected-Organization-ID", "org-2");
    if (mode === "cross-origin")
      req.headers.set("Origin", "https://other.example");
    expect(
      (await proxyMemberResources(req, "token", "actor-1")).status,
    ).toBeGreaterThanOrEqual(400);
    expect(fetch).not.toHaveBeenCalled();
  },
);
it("holds the original operation on response loss without retrying upstream", async () => {
  const fetch = vi.fn().mockRejectedValue(new Error("lost"));
  vi.stubGlobal("fetch", fetch);
  const response = await proxyMemberResources(request(), "token", "actor-1");
  expect(await response.json()).toMatchObject({
    code: "RESULT_UNVERIFIED",
    outcome: "unknown",
  });
  expect(fetch).toHaveBeenCalledTimes(1);
});
it("rejects a receipt for another member as an unknown write", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(Response.json({ ...receipt, memberId: "other" })),
  );
  const response = await proxyMemberResources(request(), "token", "actor-1");
  expect(await response.json()).toMatchObject({
    code: "RESULT_UNVERIFIED",
    outcome: "unknown",
  });
});
