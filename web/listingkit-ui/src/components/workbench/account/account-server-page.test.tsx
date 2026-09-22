import { afterEach, expect, it, vi } from "vitest";
import { AccountServerPage } from "./account-server-page";
const auth = vi.hoisted(() => ({ session: null as unknown, blocked: false }));
vi.mock("@/auth", () => ({ serverAuth: () => auth.blocked ? new Promise(() => {}) : Promise.resolve(auth.session) }));
vi.mock("next/navigation", () => ({ redirect: (url: string) => { throw new Error(`redirect:${url}`); } }));
vi.mock("./account-page", () => ({ AccountPage: () => null }));
afterEach(() => { vi.useRealTimers(); auth.session = null; auth.blocked = false; });
it.each([null, { accessToken: "token" }, { accessToken: "token", error: "RefreshAccessTokenError" }])("rejects absent or incomplete server identity", async session => {
  auth.session = session;
  await expect(AccountServerPage({ page: "profile" })).rejects.toThrow("redirect:/login?");
});
it("passes only the current subject, independent of old allowlists or membership roles", async () => {
  auth.session = { accessToken: "do-not-serialize", identityVersion: 3, identity: { tenantId: "A", userId: "u1", roles: [], userType: "zitadel" } };
  const element = await AccountServerPage({ page: "profile" });
  expect(element.props).toEqual({ page: "profile", expectedUserId: "u1" });
});
it.each([
  ["overview", "/workbench/account"],
  ["profile", "/workbench/account/profile"],
  ["profile-settings", "/workbench/account/profile/settings"],
  ["profile-business", "/workbench/account/profile/business"],
  ["profile-verification", "/workbench/account/profile/verification"],
  ["organization", "/workbench/account/organization"],
] as const)("keeps the server auth return-to route for %s", async (page, returnTo) => {
  await expect(AccountServerPage({ page })).rejects.toThrow(`redirect:/login?returnTo=${encodeURIComponent(returnTo)}`);
});

it("bounds server authentication before rendering an account leaf", async () => {
  vi.useFakeTimers();
  auth.blocked = true;
  const pending = AccountServerPage({ page: "profile-settings" });
  const assertion = expect(pending).rejects.toThrow("redirect:/login?returnTo=%2Fworkbench%2Faccount%2Fprofile%2Fsettings");
  await vi.advanceTimersByTimeAsync(15001);
  await assertion;
});
