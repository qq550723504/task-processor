import { afterEach, describe, expect, it, vi } from "vitest";
import { AccountReadError, getAccountProfile, getAccountOrganization } from "./account";

const profile = { schemaVersion: "account-v1", userId: "u1", homeOrganizationId: "A", displayName: "Alice", email: null, emailVerified: null, phoneNumber: null, phoneNumberVerified: null, source: "zitadel_userinfo", readAt: "2026-09-07T01:00:00Z" };
const organization = { schemaVersion: "account-v1", userId: "u1", homeOrganizationId: "A", effectiveOrganizationId: "B", name: "Enterprise B", roles: ["listingkit_viewer"], source: "zitadel_project_authorizations", readAt: "2026-09-07T01:00:00Z", authorizationMaxAgeSeconds: 60 };
afterEach(() => vi.unstubAllGlobals());
describe("account client", () => {
 it("reads exact personal path without organization inputs", async () => {
  const fetch = vi.fn().mockResolvedValue(Response.json(profile)); vi.stubGlobal("fetch", fetch);
  expect(await getAccountProfile({ expectedUserId: "u1" })).toEqual(profile);
  expect(fetch.mock.calls[0][0]).toBe("/api/account/profile");
  expect(fetch.mock.calls[0][1]).toMatchObject({ method: "GET", cache: "no-store" });
 });
 it("binds Effective B separately from Home A", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(organization)));
  expect((await getAccountOrganization({ expectedUserId: "u1", expectedOrganizationId: "B" })).homeOrganizationId).toBe("A");
 });
 it.each([profile, { ...profile, userId: "other" }])("rejects wrong identity", async (payload) => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(payload)));
  await expect(getAccountProfile({ expectedUserId: "new-user" })).rejects.toMatchObject({ code: "IDENTITY_CONTEXT_CHANGED" });
 });
 it("rejects old organization data", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(organization)));
  await expect(getAccountOrganization({ expectedUserId: "u1", expectedOrganizationId: "C" })).rejects.toMatchObject({ code: "ORGANIZATION_CONTEXT_CHANGED" });
 });
 it("rejects cancellation after delayed fetch", async () => {
  const controller = new AbortController();
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async () => {controller.abort(); return Response.json(profile);}));
  await expect(getAccountProfile({ expectedUserId: "u1", signal: controller.signal })).rejects.toMatchObject({ code: "DEADLINE_EXCEEDED" });
 });
 it("keeps a typed safe error instead of a provider message", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code: "DEPENDENCY_UNAVAILABLE", message: "private secret", requestId: "", fieldErrors: [] }, { status: 503 })));
  const failure = await getAccountProfile({ expectedUserId: "u1" }).catch(e => e);
  expect(failure).toBeInstanceOf(AccountReadError); expect(failure.message).not.toContain("private"); expect(failure.status).toBe(503);
 });
 it.each([{ ...profile, secret: "token" }, { ...profile, displayName: "x".repeat(513) }, { ...profile, email: "u@phone.invalid", emailVerified: true }])("rejects invalid projections", async payload => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(payload)));
  await expect(getAccountProfile({ expectedUserId: "u1" })).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
 });
 it.each([
  ["{broken", "application/json"], ["{}", "text/html"], ["x".repeat(17000), "application/json"],
 ])("distinguishes malformed wire response from transport failure", async (body, type) => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(body, { headers: { "Content-Type": type } })));
  await expect(getAccountProfile({ expectedUserId: "u1" })).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
 });
});
