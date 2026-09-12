import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { browserCaptureFixture } from "@/lib/contracts/browser-capture.fixture";
import { WorkbenchContextError } from "@/lib/api/workbench-context";
const mocks = vi.hoisted(() => ({ context: {} as Record<string, unknown>, capture: vi.fn(), read: vi.fn(), handoff: vi.fn(), status: vi.fn(), retry: vi.fn() }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => mocks.context }));
vi.mock("@/lib/api/browser-capture", () => ({ capture1688: mocks.capture, readBrowserCaptureByKey: mocks.read }));
vi.mock("./capture-handoff", async (original) => ({ ...await original<typeof import("./capture-handoff")>(), readCaptureHandoff: mocks.handoff, notifyCaptureStatus: mocks.status }));
import { CaptureReceiver } from "./capture-receiver";
const key = "22222222-2222-4222-8222-22222222222b";
const op = "11111111-1111-4111-8111-11111111111a";
const handoff = `/capture/1688#extensionId=${"a".repeat(32)}&handoffId=${op}&idempotencyKey=${key}`;
const receipt = { schemaVersion: 1, operationId: op, outcome: "published", replayed: false, productKey: "crawler:1688:981645030344", publicationId: `source-run:acquisition:${op}`, catalogVersion: "1", warnings: [], missingFacts: [] };
function fresh(userId = "actor", organizationId = "org-B") { return { user: { id: userId }, effectiveOrganizationId: organizationId, homeOrganizationId: organizationId, selectionRequired: false, organizations: [{ id: organizationId, name: "Enterprise B", roles: ["listingkit_operator"] }] }; }
beforeEach(() => {
  vi.clearAllMocks(); window.history.replaceState(null, "", handoff);
  mocks.context = { user: { id: "actor" }, effectiveOrganization: { id: "org-B", name: "Enterprise B" }, isLoading: false, isSwitching: false, error: null, blockingError: null, selectionRequired: false, retry: mocks.retry };
  mocks.retry.mockResolvedValue(fresh()); mocks.handoff.mockResolvedValue(browserCaptureFixture()); mocks.capture.mockResolvedValue(receipt); mocks.read.mockResolvedValue({ ...receipt, replayed: true }); mocks.status.mockResolvedValue(undefined);
});
afterEach(() => { vi.unstubAllGlobals(); window.history.replaceState(null, "", "/"); });
describe("Browser receiver lifecycle", () => {
  it("requires explicit current actor/enterprise confirmation and scrubs before first POST", async () => {
    mocks.capture.mockImplementation(async (intent) => { expect(window.location.hash).toBe(`#operationKey=${key}`); expect(intent.key).toBe(key); expect(intent.userId).toBe("actor"); expect(intent.organizationId).toBe("org-B"); return receipt; });
    render(<CaptureReceiver />); await screen.findByText("Browser fixture");
    expect(screen.getByText("actor")).toBeVisible(); expect(screen.getByText("Enterprise B")).toBeVisible(); expect(mocks.capture).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Confirm and submit" }));
    await screen.findByText("Published version 1"); expect(mocks.capture).toHaveBeenCalledTimes(1); expect(mocks.retry).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("button", { name: "Confirm and submit" })).not.toBeInTheDocument();
  });
  it("reload only reads the original key after explicit context confirmation", async () => {
    window.history.replaceState(null, "", `/capture/1688#operationKey=${key}`);
    render(<CaptureReceiver />); fireEvent.click(await screen.findByRole("button", { name: "Check original operation" }));
    await screen.findByText("Published version 1"); expect(mocks.handoff).not.toHaveBeenCalled(); expect(mocks.capture).not.toHaveBeenCalled();
    expect(mocks.read.mock.calls[0]!.slice(0, 2)).toEqual([{ userId: "actor", organizationId: "org-B" }, key]);
  });
  it("worker death after receipt does not prevent the frozen submission", async () => {
    mocks.status.mockResolvedValue(undefined); render(<CaptureReceiver />); await screen.findByText("Browser fixture");
    mocks.handoff.mockRejectedValue(new Error("worker gone")); fireEvent.click(screen.getByRole("button", { name: "Confirm and submit" }));
    await screen.findByText("Published version 1"); expect(mocks.handoff).toHaveBeenCalledTimes(1); expect(mocks.capture).toHaveBeenCalledTimes(1);
  });
  it.each(["org", "actor", "revoked"])("does not dispatch on fresh %s context drift", async (mode) => {
    mocks.retry.mockResolvedValue(mode === "revoked" ? null : fresh(mode === "actor" ? "other" : "actor", mode === "org" ? "org-C" : "org-B"));
    render(<CaptureReceiver />); await screen.findByText("Browser fixture"); fireEvent.click(screen.getByRole("button", { name: "Confirm and submit" }));
    await screen.findByText(/Context changed/); expect(mocks.capture).not.toHaveBeenCalled(); expect(window.location.hash).toBe(`#operationKey=${key}`);
  });
  it("lost POST response is unknown and manual recovery never POSTs again", async () => {
    mocks.capture.mockRejectedValue(new WorkbenchContextError(503, "OUTCOME_UNKNOWN", "", [])); render(<CaptureReceiver />); await screen.findByText("Browser fixture"); fireEvent.click(screen.getByRole("button", { name: "Confirm and submit" }));
    await screen.findByText(/Outcome is unknown/); fireEvent.click(screen.getByRole("button", { name: "Check original operation" })); await screen.findByText("Published version 1"); expect(mocks.capture).toHaveBeenCalledTimes(1); expect(mocks.read).toHaveBeenCalledTimes(1);
  });
  it("cancellation retains key and does not render a late success", async () => {
    let finish!: (value: typeof receipt) => void; mocks.capture.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    render(<CaptureReceiver />); await screen.findByText("Browser fixture"); fireEvent.click(screen.getByRole("button", { name: "Confirm and submit" })); await waitFor(() => expect(mocks.capture).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole("button", { name: "Stop waiting" })); await act(async () => finish(receipt));
    expect(screen.queryByText("Published version 1")).not.toBeInTheDocument(); expect(window.location.hash).toBe(`#operationKey=${key}`); expect(mocks.capture).toHaveBeenCalledTimes(1);
  });
  it("enterprise switch aborts old work and hides the old receipt", async () => {
    let finish!: (value: typeof receipt) => void; mocks.capture.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    const view = render(<CaptureReceiver />); await screen.findByText("Browser fixture"); fireEvent.click(screen.getByRole("button", { name: "Confirm and submit" })); await waitFor(() => expect(mocks.capture).toHaveBeenCalledTimes(1));
    mocks.context = { ...mocks.context, effectiveOrganization: { id: "org-C", name: "Enterprise C" } }; view.rerender(<CaptureReceiver />); await act(async () => finish(receipt));
    expect(mocks.capture.mock.calls[0]![1].aborted).toBe(true); expect(screen.queryByText("Published version 1")).not.toBeInTheDocument(); expect(screen.getByRole("button", { name: "Check original operation" })).toBeDisabled();
  });
  it("renders evidence as text, never untrusted HTML", async () => {
    mocks.handoff.mockResolvedValue({ ...browserCaptureFixture(), evidence: { ...browserCaptureFixture().evidence, title: '<img src=x onerror="alert(1)">' } });
    render(<CaptureReceiver />); await screen.findByText('<img src=x onerror="alert(1)">'); expect(document.querySelector("img")).toBeNull();
  });
});
