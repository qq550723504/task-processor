import { describe, it, expect } from "vitest";
import { loadDataIntent, saveDataIntent } from "./data-services";
describe("data command recovery", () => {
    it("retains the original command and scope without retaining key secrets", () => {
        const scope = { userId: "user", organizationId: "org" };
        const intent = { ...scope, key: "a233d58b-1fd3-40d7-a983-d35bbec45313", path: "keys", body: { name: "key", expiresAt: "2026-10-20T00:00:00Z", dailyRows: 20, monthlyCostFen: 100, permissions: ["amazon.acquire"] }, specialist: false };
        expect(saveDataIntent(intent)).toBe(true);
        expect(loadDataIntent(scope, false)).toEqual(intent);
        expect(loadDataIntent({ userId: "another", organizationId: "org" }, false)).toBeNull();
        expect(saveDataIntent({ ...intent, body: { ...intent.body, secret: "never-store" } })).toBe(false);
    });
});
