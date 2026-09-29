import { beforeEach, expect, it } from "vitest";
import {
  clearStoreAuthorization,
  readStoreAuthorization,
  rememberStoreAuthorization,
} from "./store-connection";
const original = {
  expectedUserId: "user",
  expectedOrganizationId: "org-1",
  storeId: "11111111-1111-4111-8111-11111111111a",
  attemptId: "22222222-2222-4222-8222-22222222222b",
  expiresAt: "2026-09-29T00:00:00Z",
};
beforeEach(() => sessionStorage.clear());
it("clears only the exact original authorization and retains a newer attempt", () => {
  rememberStoreAuthorization(original);
  const newer = {
    ...original,
    expectedOrganizationId: "org-2",
    attemptId: "33333333-3333-4333-8333-33333333333c",
  };
  rememberStoreAuthorization(newer);
  clearStoreAuthorization(original);
  expect(readStoreAuthorization()).toEqual(newer);
  clearStoreAuthorization(newer);
  expect(readStoreAuthorization()).toBeNull();
});
