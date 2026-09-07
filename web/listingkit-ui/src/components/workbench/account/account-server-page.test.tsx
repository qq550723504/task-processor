import { afterEach, expect, it, vi } from "vitest";
import { AccountServerPage } from "./account-server-page";
const auth = vi.hoisted(() => ({ session: null as unknown }));
vi.mock("@/auth", () => ({ serverAuth: () => Promise.resolve(auth.session) }));
vi.mock("next/navigation", () => ({ redirect: (url: string) => { throw new Error(`redirect:${url}`); } }));
vi.mock("./account-page", () => ({ AccountPage: () => null }));
afterEach(() => { auth.session = null; });
it.each([null, { accessToken: "token" }, { accessToken: "token", error: "RefreshAccessTokenError" }])("rejects absent or incomplete server identity", async session => {
  auth.session = session;
  await expect(AccountServerPage({ page: "profile" })).rejects.toThrow("redirect:/login?");
});
it("passes only the current subject, independent of old allowlists or membership roles", async () => {
  auth.session = { accessToken: "do-not-serialize", identityVersion: 3, identity: { tenantId: "A", userId: "u1", roles: [], userType: "zitadel" } };
  const element = await AccountServerPage({ page: "profile" });
  expect(element.props).toEqual({ page: "profile", expectedUserId: "u1" });
});
