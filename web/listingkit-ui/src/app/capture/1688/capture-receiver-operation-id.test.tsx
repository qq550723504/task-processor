import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { CaptureReceiver } from "./capture-receiver";

const fixture = vi.hoisted(() => ({
  capture: vi.fn(), byKey: vi.fn(), byId: vi.fn(), entry: vi.fn(),
  context: { isLoading: false, isSwitching: false, selectionRequired: false, error: null, blockingError: null,
    user: { id: "actor" }, effectiveOrganization: { id: "B", name: "Fixture B" }, retry: vi.fn() },
}));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => fixture.context }));
vi.mock("@/lib/api/browser-capture", () => ({ capture1688: fixture.capture, readBrowserCaptureByKey: fixture.byKey, readBrowserCapture: fixture.byId }));
vi.mock("./capture-handoff", () => ({ parseCaptureEntry: fixture.entry, readCaptureHandoff: vi.fn(), notifyCaptureStatus: vi.fn() }));
const key = "10000000-0000-4000-8000-000000000399";
const operationId = "20000000-0000-4000-8000-000000000399";
const receipt = { outcome: "published", operationId, catalogVersion: "1", productKey: "crawler:1688:981645030344", publicationId: "synthetic-publication", warnings: [], missingFacts: [] };
beforeEach(() => {
  vi.resetAllMocks();
  window.history.replaceState(null, "", `/capture/1688#operationKey=${key}`);
  fixture.entry.mockReturnValue({ kind: "recovery", key });
  fixture.context.effectiveOrganization = { id: "B", name: "Fixture B" };
  fixture.context.retry.mockResolvedValue({ selectionRequired: false, user: { id: "actor" }, effectiveOrganizationId: "B" });
  fixture.byKey.mockResolvedValue(receipt);
  fixture.byId.mockResolvedValue(receipt);
});
afterEach(cleanup);
async function check() {
  fireEvent.click(await screen.findByRole("button", { name: "Check original operation" }));
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Published version 1"));
}
it("uses the verified receipt id only after original-key recovery, without another POST", async () => {
  render(<CaptureReceiver />);
  await check();
  expect(fixture.byKey).toHaveBeenCalledWith({ userId: "actor", organizationId: "B" }, key, expect.any(AbortSignal));
  await check();
  expect(fixture.byId).toHaveBeenCalledWith({ userId: "actor", organizationId: "B" }, operationId, expect.any(AbortSignal));
  expect(fixture.byKey).toHaveBeenCalledTimes(1);
  expect(fixture.capture).not.toHaveBeenCalled();
  expect(window.location.hash).toBe(`#operationKey=${key}`);
});
it("reload discards the in-memory receipt id and recovers only by original key", async () => {
  const first = render(<CaptureReceiver />);
  await check();
  first.unmount();
  render(<CaptureReceiver />);
  await check();
  expect(fixture.byKey).toHaveBeenCalledTimes(2);
  expect(fixture.byId).not.toHaveBeenCalled();
  expect(fixture.capture).not.toHaveBeenCalled();
});
it("does not use a prior receipt id after the effective enterprise changes", async () => {
  const view = render(<CaptureReceiver />);
  await check();
  fixture.context.effectiveOrganization = { id: "C", name: "Fixture C" };
  view.rerender(<CaptureReceiver />);
  expect(screen.queryByText("synthetic-publication")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Check original operation" }));
  expect(fixture.byId).not.toHaveBeenCalled();
  expect(fixture.byKey).toHaveBeenCalledTimes(1);
  expect(fixture.capture).not.toHaveBeenCalled();
});
