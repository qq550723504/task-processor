import { afterEach, describe, expect, it, vi } from "vitest";
import { getAccountIdentityProfile, setAccountEmail, updateAccountIdentityProfile, verifyAccountPhone } from "./account-identity";

const operation = { schemaVersion: "account-identity-operation-v1", operation: "email", state: "verification_pending", source: "zitadel_auth_v1" };
const profile = { schemaVersion: "account-identity-profile-v1", userId: "u1", firstName: "First", lastName: "Last", nickName: "", displayName: "Name", preferredLanguage: "", gender: "", source: "zitadel_auth_v1" };

describe("account identity api", () => {
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); });

  it("updates an email through the Shuomi account BFF", async () => {
    const fetcher = vi.fn().mockResolvedValue(Response.json(operation));
    vi.stubGlobal("fetch", fetcher);
    await expect(setAccountEmail({ expectedUserId: "u1", email: "user@example.test" })).resolves.toEqual(operation);
    expect(fetcher).toHaveBeenCalledWith("/api/account/identity/email", expect.objectContaining({ method: "PUT", body: JSON.stringify({ email: "user@example.test" }) }));
  });

  it("sends phone verification codes to the official identity service", async () => {
    const fetcher = vi.fn().mockResolvedValue(Response.json({ ...operation, operation: "phone/verify", state: "verified" }));
    vi.stubGlobal("fetch", fetcher);
    await expect(verifyAccountPhone({ expectedUserId: "u1", code: "123456" })).resolves.toMatchObject({ state: "verified" });
    expect(fetcher).toHaveBeenCalledWith("/api/account/identity/phone/verify", expect.objectContaining({ method: "POST", body: JSON.stringify({ code: "123456" }) }));
  });

  it("reads and updates the ZITADEL profile through the Shuomi account BFF", async () => {
    const fetcher = vi.fn()
      .mockResolvedValueOnce(Response.json(profile))
      .mockResolvedValueOnce(Response.json({ ...operation, operation: "profile", state: "updated" }));
    vi.stubGlobal("fetch", fetcher);
    await expect(getAccountIdentityProfile({ expectedUserId: "u1" })).resolves.toEqual(profile);
    await expect(updateAccountIdentityProfile({ expectedUserId: "u1", input: { firstName: "New", lastName: "Last", nickName: "N", displayName: "New Last", preferredLanguage: "", gender: "" } })).resolves.toMatchObject({ operation: "profile" });
    expect(fetcher.mock.calls[0][0]).toBe("/api/account/identity/profile");
    expect(fetcher.mock.calls[0][1]).toMatchObject({ method: "GET" });
    expect(fetcher.mock.calls[1][0]).toBe("/api/account/identity/profile");
    expect(fetcher.mock.calls[1][1]).toMatchObject({ method: "PUT", body: JSON.stringify({ firstName: "New", lastName: "Last", nickName: "N", displayName: "New Last", preferredLanguage: "", gender: "" }) });
  });

  it("marks a mutation unknown when the browser deadline fires after dispatch", async () => {
    vi.useFakeTimers();
    vi.stubGlobal("fetch", vi.fn((_: unknown, init: RequestInit) => new Promise<Response>((_, reject) => {
      init.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")), { once: true });
    })));
    const pending = setAccountEmail({ expectedUserId: "u1", email: "user@example.test" });
    const outcome = expect(pending).rejects.toMatchObject({ status: 504, code: "RESULT_UNVERIFIED" });
    await vi.advanceTimersByTimeAsync(15000);
    await outcome;
  });

  it("marks a mutation unknown when the browser loses the dispatched response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("connection reset")));
    await expect(setAccountEmail({ expectedUserId: "u1", email: "user@example.test" })).rejects.toMatchObject({ status: 502, code: "RESULT_UNVERIFIED" });
  });

  it("marks a mutation unknown when the dispatched BFF response cannot be parsed", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("not-json", { status: 200, headers: { "Content-Type": "application/json" } })));
    await expect(setAccountEmail({ expectedUserId: "u1", email: "user@example.test" })).rejects.toMatchObject({ status: 502, code: "RESULT_UNVERIFIED" });
  });
});
