import { afterEach, describe, expect, it, vi } from "vitest";
import { parseCaptureEntry, readCaptureHandoff, notifyCaptureStatus } from "./capture-handoff";
import { browserCaptureFixture } from "@/lib/contracts/browser-capture.fixture";
const key = "22222222-2222-4222-8222-22222222222b";
const handoffId = "11111111-1111-4111-8111-11111111111a";
const extensionId = "a".repeat(32);
const url = `http://localhost/capture/1688#extensionId=${extensionId}&handoffId=${handoffId}&idempotencyKey=${key}`;
afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers(); });
describe("frozen external handoff", () => {
  it("admits exact handoff or recovery fragment without authority", () => {
    expect(parseCaptureEntry(url)).toEqual({ kind: "handoff", extensionId, handoffId, key });
    expect(parseCaptureEntry(`http://localhost/capture/1688#operationKey=${key}`)).toEqual({ kind: "recovery", key });
  });
  it.each([`${url}&orgId=org`, `${url}&handoffId=${handoffId}`, url.replace("/1688#", "/1688/#"), url.replace("/1688#", "/1688?x=1#"), url.replace(extensionId, "bad"), `${url}&operationKey=${key}`, `http://localhost/capture/1688#operationKey=${key}&payload=x`])("rejects a non-contract envelope %#", (input) => expect(parseCaptureEntry(input)).toBeNull());
  it("validates response nonce/key and exact message schema", async () => {
    const entry = parseCaptureEntry(url)!; if (entry.kind !== "handoff") throw new Error("fixture");
    const sendMessage = vi.fn((_id, message, callback) => callback({ version: 1, type: "capture.payload", handoffId, idempotencyKey: key, payload: browserCaptureFixture() }));
    vi.stubGlobal("chrome", { runtime: { sendMessage } });
    expect(await readCaptureHandoff(entry, new AbortController().signal)).toEqual(browserCaptureFixture());
    expect(sendMessage.mock.calls[0]![0]).toBe(extensionId);
    expect(sendMessage.mock.calls[0]![1]).toEqual({ version: 1, type: "capture.read", handoffId, idempotencyKey: key });
  });
  it("uses at most three reads for the tab binding race, never a new key", async () => {
    vi.useFakeTimers(); const entry = parseCaptureEntry(url)!; if (entry.kind !== "handoff") throw new Error("fixture");
    const sendMessage = vi.fn((_id, _message, callback) => callback(undefined)); vi.stubGlobal("chrome", { runtime: { sendMessage } });
    const pending = readCaptureHandoff(entry, new AbortController().signal).catch(() => null);
    await vi.runAllTimersAsync(); expect(await pending).toBeNull(); expect(sendMessage).toHaveBeenCalledTimes(3);
    for (const call of sendMessage.mock.calls) expect(call[1].idempotencyKey).toBe(key);
  });
  it("status delivery failure is best-effort, never proof or a reason to resubmit", async () => {
    const entry = parseCaptureEntry(url)!; if (entry.kind !== "handoff") throw new Error("fixture");
    await expect(notifyCaptureStatus(entry, "outcome_unknown")).resolves.toBeUndefined();
  });
});
