import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { AcquisitionAPIError } from "@/lib/api/product-acquisition";
import * as api from "@/lib/api/acquisition-main-image";
import { useMainImageFlow } from "./use-main-image-flow";

vi.mock("@/lib/api/acquisition-main-image");
const operationId = "11111111-1111-4111-8111-111111111111";
const runId = "22222222-2222-4222-8222-222222222222";
const key = "33333333-3333-4333-8333-333333333333";
const actionId = "44444444-4444-4444-8444-444444444444";
const scope = { userId: "actor", organizationId: "organization" };
const storageKey = `main-image:${JSON.stringify([scope.userId, scope.organizationId, operationId])}`;
const candidate = { id: "catalog-image-1", displayUrl: "https://example.com/source.png" };
const ready = { runId, status: "awaiting_final_approval" as const, planRevision: 1, resultDigest: "digest", approvalAvailable: true, imageUrl: "https://example.com/output.png" };
const approval = { planRevision: 1, resultDigest: "digest", actionId };
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>((done) => { resolve = done; }); return { promise, resolve }; }
beforeEach(() => {
  vi.resetAllMocks(); sessionStorage.clear();
  vi.mocked(api.readMainImageCandidates).mockResolvedValue({ operationId, candidates: [candidate] });
  vi.mocked(api.startMainImage).mockResolvedValue({ runId, status: "accepted" });
  vi.mocked(api.readMainImage).mockResolvedValue(ready);
  vi.mocked(api.approveMainImage).mockResolvedValue({ runId, status: "accepted" });
});
afterEach(() => vi.restoreAllMocks());

it("retains the original Start key after response loss and remount", async () => {
  vi.mocked(api.startMainImage).mockRejectedValueOnce(new AcquisitionAPIError("OUTCOME_UNKNOWN", 503));
  const first = renderHook(() => useMainImageFlow({ operationId, scope }));
  await waitFor(() => expect(first.result.current.candidates).toHaveLength(1));
  act(() => first.result.current.select(candidate.id));
  await act(() => first.result.current.start());
  const original = vi.mocked(api.startMainImage).mock.calls[0][2];
  expect(first.result.current.error).toBe("OUTCOME_UNKNOWN");
  first.unmount();
  const second = renderHook(() => useMainImageFlow({ operationId, scope }));
  await waitFor(() => expect(second.result.current.candidates).toHaveLength(1));
  await act(() => second.result.current.start());
  expect(vi.mocked(api.startMainImage).mock.calls[1][2]).toBe(original);
  expect(second.result.current.intent?.runId).toBe(runId);
});

it("does not dispatch when original request identity cannot be saved", async () => {
  const { result } = renderHook(() => useMainImageFlow({ operationId, scope }));
  await waitFor(() => expect(result.current.candidates).toHaveLength(1));
  act(() => result.current.select(candidate.id));
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("quota"); });
  await act(async () => { await expect(result.current.start()).resolves.toBeUndefined(); });
  expect(api.startMainImage).not.toHaveBeenCalled();
  expect(result.current.error).toBe("LOCAL_STORAGE_UNAVAILABLE");
});

it("ignores old organization responses and does not carry its intent into the new scope", async () => {
  const old = deferred<{ operationId: string; candidates: typeof candidate[] }>();
  vi.mocked(api.readMainImageCandidates).mockReturnValueOnce(old.promise);
  sessionStorage.setItem(storageKey, JSON.stringify({ sourceImageId: candidate.id, key, runId }));
  const { result, rerender } = renderHook(({ current }) => useMainImageFlow({ operationId, scope: current }), { initialProps: { current: scope } });
  await waitFor(() => expect(result.current.result?.runId).toBe(runId));
  rerender({ current: { ...scope, organizationId: "other" } });
  await waitFor(() => expect(api.readMainImageCandidates).toHaveBeenCalledTimes(2));
  await act(async () => { old.resolve({ operationId, candidates: [{ ...candidate, id: "catalog-image-9" }] }); });
  expect(result.current.candidates).toEqual([candidate]);
  expect(result.current.intent).toBeNull();
  expect(result.current.result).toBeNull();
  expect(result.current.selected).toBe("");
});

it("verifies a lost approval with the original action even after the workflow is completed", async () => {
  sessionStorage.setItem(storageKey, JSON.stringify({ sourceImageId: candidate.id, key, runId, approval }));
  vi.mocked(api.readMainImage).mockResolvedValue({ ...ready, status: "completed", approvalAvailable: false });
  const { result } = renderHook(() => useMainImageFlow({ operationId, scope }));
  await waitFor(() => expect(result.current.result?.status).toBe("completed"));
  await act(() => result.current.approve());
  expect(api.approveMainImage).toHaveBeenCalledWith(operationId, runId, approval, scope, expect.any(AbortSignal));
});

it("does not let a late read reveal a result after live permission denial", async () => {
  const pending = deferred<typeof ready>();
  sessionStorage.setItem(storageKey, JSON.stringify({ sourceImageId: candidate.id, key, runId }));
  vi.mocked(api.readMainImageCandidates).mockRejectedValue(new AcquisitionAPIError("FORBIDDEN", 403));
  vi.mocked(api.readMainImage).mockReturnValue(pending.promise);
  const { result } = renderHook(() => useMainImageFlow({ operationId, scope }));
  await waitFor(() => expect(result.current.error).toBe("FORBIDDEN"));
  await act(async () => { pending.resolve(ready); });
  expect(result.current.result).toBeNull();
  await act(() => result.current.approve());
  expect(api.approveMainImage).not.toHaveBeenCalled();
});

it("retains the original command when saving an accepted run fails", async () => {
  const { result } = renderHook(() => useMainImageFlow({ operationId, scope }));
  await waitFor(() => expect(result.current.candidates).toHaveLength(1));
  act(() => result.current.select(candidate.id));
  const save = Storage.prototype.setItem;
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(function (this: Storage, name, value) {
    if (JSON.parse(value).runId) throw new Error("quota after ACK");
    return save.call(this, name, value);
  });
  await act(() => result.current.start());
  const original = vi.mocked(api.startMainImage).mock.calls[0][2];
  expect(result.current.error).toBe("LOCAL_STORAGE_UNAVAILABLE");
  expect(result.current.intent?.key).toBe(original);
  expect(api.readMainImage).not.toHaveBeenCalled();
  vi.restoreAllMocks();
  await act(() => result.current.start());
  expect(vi.mocked(api.startMainImage).mock.calls[1][2]).toBe(original);
});

it.each(["read", "corrupt"])("blocks new mutations when local identity is unavailable: %s", async (kind) => {
  if (kind === "read") vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("denied"); });
  else sessionStorage.setItem(storageKey, '{"key":"unreadable"}');
  const { result } = renderHook(() => useMainImageFlow({ operationId, scope }));
  await waitFor(() => expect(result.current.candidates).toHaveLength(1));
  act(() => result.current.select(candidate.id));
  await act(() => result.current.start());
  expect(api.startMainImage).not.toHaveBeenCalled();
  expect(result.current.error).toBe("LOCAL_STORAGE_UNAVAILABLE");
});

it("ignores an accepted mutation from the old scope without losing its original stored key", async () => {
  const pending = deferred<{ runId: string; status: "accepted" }>();
  vi.mocked(api.startMainImage).mockReturnValue(pending.promise);
  const { result, rerender } = renderHook(({ current }) => useMainImageFlow({ operationId, scope: current }), { initialProps: { current: scope } });
  await waitFor(() => expect(result.current.candidates).toHaveLength(1));
  act(() => result.current.select(candidate.id));
  let submission!: Promise<void>;
  act(() => { submission = result.current.start(); });
  const original = JSON.parse(sessionStorage.getItem(storageKey)!);
  rerender({ current: { ...scope, organizationId: "other" } });
  await act(async () => { pending.resolve({ runId, status: "accepted" }); await submission; });
  expect(result.current.intent).toBeNull();
  expect(result.current.result).toBeNull();
  expect(api.readMainImage).not.toHaveBeenCalled();
  expect(JSON.parse(sessionStorage.getItem(storageKey)!)).toEqual(original);
  rerender({ current: scope });
  await waitFor(() => expect(result.current.intent?.key).toBe(original.key));
  expect(api.startMainImage).toHaveBeenCalledTimes(1); // remount only reads
});

it("keeps the same approval action through an ambiguous response and explicit retry", async () => {
  sessionStorage.setItem(storageKey, JSON.stringify({ sourceImageId: candidate.id, key, runId }));
  vi.mocked(api.approveMainImage).mockRejectedValueOnce(new AcquisitionAPIError("OUTCOME_UNKNOWN", 503));
  const { result } = renderHook(() => useMainImageFlow({ operationId, scope }));
  await waitFor(() => expect(result.current.result).toEqual(ready));
  await act(() => result.current.approve());
  const original = vi.mocked(api.approveMainImage).mock.calls[0][2];
  expect(result.current.approvalAcknowledged).toBe(false);
  expect(result.current.error).toBe("OUTCOME_UNKNOWN");
  vi.mocked(api.readMainImage).mockResolvedValue({ ...ready, status: "completed", approvalAvailable: false });
  await act(() => result.current.refresh());
  expect(result.current.approvalAcknowledged).toBe(false);
  await act(() => result.current.approve());
  expect(vi.mocked(api.approveMainImage).mock.calls[1][2]).toEqual(original);
  expect(result.current.approvalAcknowledged).toBe(true);
});

it("does not POST approval when its action cannot be stored or its payload changed", async () => {
  sessionStorage.setItem(storageKey, JSON.stringify({ sourceImageId: candidate.id, key, runId, approval }));
  const { result } = renderHook(() => useMainImageFlow({ operationId, scope }));
  await waitFor(() => expect(result.current.result).toEqual(ready));
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("quota"); });
  await act(() => result.current.approve());
  expect(result.current.error).toBe("LOCAL_STORAGE_UNAVAILABLE");
  expect(api.approveMainImage).not.toHaveBeenCalled();
  vi.restoreAllMocks();
  vi.mocked(api.readMainImage).mockResolvedValue({ ...ready, resultDigest: "changed" });
  await act(() => result.current.refresh());
  await act(() => result.current.approve());
  expect(result.current.error).toBe("IMAGE_CONFLICT");
  expect(api.approveMainImage).not.toHaveBeenCalled();
});

it("coalesces double clicks without automatically retrying an in-flight Start", async () => {
  const pending = deferred<{ runId: string; status: "accepted" }>();
  vi.mocked(api.startMainImage).mockReturnValue(pending.promise);
  const { result } = renderHook(() => useMainImageFlow({ operationId, scope }));
  await waitFor(() => expect(result.current.candidates).toHaveLength(1));
  act(() => result.current.select(candidate.id));
  let first!: Promise<void>;
  act(() => { first = result.current.start(); void result.current.start(); });
  expect(api.startMainImage).toHaveBeenCalledOnce();
  await act(async () => { pending.resolve({ runId, status: "accepted" }); await first; });
});
