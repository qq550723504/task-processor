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
    mocks.retry.mockResolvedValue(fresh()); fireEvent.click(screen.getByRole("button", { name: "Confirm and submit" }));
    await screen.findByText("Published version 1"); expect(mocks.capture).toHaveBeenCalledTimes(1);
  });
  it("restores submission when canceled before capture dispatch", async () => {
    let fail!: (reason?: unknown) => void;
    mocks.retry.mockImplementation(() => new Promise((_resolve, reject) => { fail = reject; }));
    render(<CaptureReceiver />); await screen.findByText("Browser fixture");
    fireEvent.click(screen.getByRole("button", { name: "Confirm and submit" }));
    await waitFor(() => expect(mocks.retry).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole("button", { name: "Stop waiting" }));
    await act(async () => fail(new DOMException("Aborted", "AbortError")));
    expect(mocks.capture).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Confirm and submit" })).toBeInTheDocument();
  });
  it("shows definitive pre-admission rejection instead of unknown recovery", async () => {
    mocks.capture.mockRejectedValue(new WorkbenchContextError(400, "INVALID_ACQUISITION", "", []));
    render(<CaptureReceiver />); await screen.findByText("Browser fixture");
    fireEvent.click(screen.getByRole("button", { name: "Confirm and submit" }));
    await screen.findByText(/rejected before admission/);
    expect(screen.queryByRole("button", { name: "Check original operation" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Confirm and submit" })).not.toBeInTheDocument();
    expect(mocks.status).toHaveBeenCalledWith(expect.anything(), "failed");
  });
  it("restores submission after capacity rejection without creating recovery state", async () => {
    mocks.capture.mockRejectedValue(new WorkbenchContextError(429, "ACQUISITION_CAPACITY", "", []));
    render(<CaptureReceiver />); await screen.findByText("Browser fixture");
    fireEvent.click(screen.getByRole("button", { name: "Confirm and submit" }));
    await screen.findByText(/capacity is full/);
    expect(screen.queryByRole("button", { name: "Check original operation" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Confirm and submit" })).toBeInTheDocument();
    expect(mocks.status).toHaveBeenCalledWith(expect.anything(), "failed");
  });
  it("restores submission after a pre-dispatch deadline", async () => {
    mocks.capture.mockRejectedValue(new WorkbenchContextError(504, "DEADLINE_EXCEEDED", "", []));
    render(<CaptureReceiver />); await screen.findByText("Browser fixture");
    fireEvent.click(screen.getByRole("button", { name: "Confirm and submit" }));
    await screen.findByText(/not submitted before the deadline/);
    expect(screen.queryByRole("button", { name: "Check original operation" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Confirm and submit" })).toBeInTheDocument();
    expect(window.location.hash).toBe(`#operationKey=${key}`);
    expect(mocks.status).not.toHaveBeenCalledWith(expect.anything(), "outcome_unknown");
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

// The batch executor drives this page without reading its prose. It reads one DOM
// contract: the verified scope, the real confirmation control, the page's own refusal
// (design section 4 D1.2 guard 3), and a terminal result it may record (section 15.10).
// These tests pin that contract to the exact selectors internal/batchcapture/apppage.go
// queries.
describe("Executor-readable application contract", () => {
  const node = (selector: string) => document.querySelector(selector);
  // The containers are structural: they must exist whether or not they carry a value,
  // so an assertion that nothing was published cannot pass by the contract being absent.
  const value = (selector: string, attribute?: string) => {
    const found = node(selector);
    expect(found).not.toBeNull();
    return attribute ? found?.getAttribute(attribute) : found?.textContent;
  };
  const result = () => value("[data-batch-submit-result]", "data-batch-submit-result");
  const refusal = () => value("[data-batch-submit-refusal]");
  const submit = async () => {
    render(<CaptureReceiver />); await screen.findByText("Browser fixture");
    fireEvent.click(screen.getByRole("button", { name: "Confirm and submit" }));
  };
  it("marks a handoff the page could not read as a refusal and never a result", async () => {
    // The payload never arrived, so this page rendered no control at all. Without the
    // refusal the executor could only poll its readiness contract to the deadline and then
    // record a generic transfer failure for a handoff the page already explained.
    mocks.handoff.mockRejectedValue(new Error("expired"));
    render(<CaptureReceiver />); await screen.findByText(/Browser handoff expired/);
    expect(refusal()).toBe("HANDOFF_UNREADABLE");
    expect(result()).toBe("");
    expect(node("[data-batch-confirm-submit]")).toBeNull();
  });
  it("marks refusing to submit without a retained recovery key as a refusal", async () => {
    // The page declined before dispatch because it could not keep the key that a person
    // would need to recover the operation. Nothing was submitted, and the executor must not
    // treat this as an unknown outcome it can retry.
    const replace = vi.spyOn(window.history, "replaceState").mockImplementation(() => { throw new Error("storage blocked"); });
    try {
      await submit();
      await screen.findByText(/Recovery key could not be retained/);
      expect(refusal()).toBe("OPERATION_KEY_UNRETAINED");
      expect(result()).toBe("");
    } finally { replace.mockRestore(); }
  });
  it("does not claim a refusal when the page cannot know whether anything was submitted", async () => {
    // A malformed or expired handoff fragment is the one branch that must stay unmarked:
    // the page cannot say a request was never submitted, and a refusal would tell the
    // executor to stop for a reason the page does not actually have.
    window.history.replaceState(null, "", "/capture/1688#notAHandoff=1");
    render(<CaptureReceiver />); await screen.findByText(/invalid or expired/);
    expect(refusal()).toBe("");
    expect(result()).toBe("");
  });
  it("exposes the verified scope and marks the control the executor must click", async () => {
    render(<CaptureReceiver />); await screen.findByText("Browser fixture");
    expect(node("[data-batch-scope-actor]")?.textContent).toBe("actor");
    expect(node("[data-batch-scope-organization]")?.textContent).toBe("org-B");
    // The executor resolves the control with a first-match query, so exactly one node may
    // carry the marker; a second one would make which control gets clicked ambiguous.
    expect(document.querySelectorAll("[data-batch-confirm-submit]")).toHaveLength(1);
    expect(node("[data-batch-confirm-submit]")).toBe(screen.getByRole("button", { name: "Confirm and submit" }));
    expect(screen.getByRole("button", { name: "Confirm and submit" })).toBeEnabled();
  });
  it("never marks a control the executor could mistake for the submit control", async () => {
    let finish!: (value: typeof receipt) => void;
    mocks.capture.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    render(<CaptureReceiver />); await screen.findByText("Browser fixture");
    fireEvent.click(screen.getByRole("button", { name: "Confirm and submit" }));
    await waitFor(() => expect(mocks.capture).toHaveBeenCalledTimes(1));
    expect(screen.getByRole("button", { name: "Stop waiting" })).toBeVisible();
    expect(document.querySelectorAll("[data-batch-confirm-submit]")).toHaveLength(0);
    await act(async () => finish(receipt));
  });
  it("publishes no scope contract while the application has not verified a context", async () => {
    mocks.context = { ...mocks.context, isLoading: true };
    render(<CaptureReceiver />); await screen.findByText("Browser fixture");
    expect(node("[data-batch-scope-actor]")).toBeNull();
    expect(node("[data-batch-scope-organization]")).toBeNull();
    expect(screen.getByRole("button", { name: "Confirm and submit" })).toBeDisabled();
  });
  it("records a terminal published result with its operation id and no refusal", async () => {
    await submit();
    expect(result()).toBe(""); expect(refusal()).toBe("");
    await screen.findByText("Published version 1");
    expect(result()).toBe("published");
    expect(node("[data-batch-submit-result]")?.getAttribute("data-batch-operation-id")).toBe(op);
    expect(refusal()).toBe("");
  });
  it("records a terminal failure as a terminal result, not as a refusal", async () => {
    mocks.capture.mockResolvedValue({ ...receipt, outcome: "failed", productKey: undefined, publicationId: undefined, catalogVersion: undefined });
    await submit(); await screen.findByText(/recorded this operation as failed/);
    expect(result()).toBe("failed"); expect(refusal()).toBe("");
  });
  it("never publishes an in-flight read as a terminal result", async () => {
    mocks.read.mockResolvedValue({ ...receipt, outcome: "acquiring", productKey: undefined, publicationId: undefined, catalogVersion: undefined });
    window.history.replaceState(null, "", `/capture/1688#operationKey=${key}`);
    render(<CaptureReceiver />); fireEvent.click(await screen.findByRole("button", { name: "Check original operation" }));
    await screen.findByText(/Outcome is unknown/);
    expect(result()).toBe(""); expect(refusal()).toBe("");
  });
  it.each([
    ["CONTEXT_UNAVAILABLE", "retry-drift", /Context changed/],
    ["ACQUISITION_CAPACITY", "ACQUISITION_CAPACITY", /capacity is full/],
    ["DEADLINE_EXCEEDED", "DEADLINE_EXCEEDED", /not submitted before the deadline/],
    ["INVALID_ACQUISITION", "INVALID_ACQUISITION", /rejected before admission/],
    ["SOURCE_TOO_LARGE", "SOURCE_TOO_LARGE", /rejected before admission/],
    ["NOT_DISPATCHED", "retry-unknown", /was not submitted/],
  ])("marks the page's own %s refusal so the executor never retries it", async (code, mode, message) => {
    if (mode === "retry-drift") mocks.retry.mockResolvedValue(fresh("other"));
    else if (mode === "retry-unknown") mocks.retry.mockRejectedValue(new WorkbenchContextError(503, "OUTCOME_UNKNOWN", "", []));
    else mocks.capture.mockRejectedValue(new WorkbenchContextError(400, mode, "", []));
    await submit(); await screen.findByText(message);
    expect(refusal()).toBe(code);
    expect(result()).toBe("");
  });
  it("leaves an indeterminate outcome unmarked so the executor records unknown", async () => {
    mocks.capture.mockRejectedValue(new WorkbenchContextError(503, "OUTCOME_UNKNOWN", "", []));
    await submit(); await screen.findByText(/Outcome is unknown/);
    expect(result()).toBe(""); expect(refusal()).toBe("");
  });
  it("clears the refusal when the operator restores context and asks again", async () => {
    mocks.retry.mockResolvedValue(fresh("other"));
    await submit(); await screen.findByText(/Context changed/);
    expect(refusal()).toBe("CONTEXT_UNAVAILABLE");
    mocks.retry.mockResolvedValue(fresh());
    fireEvent.click(screen.getByRole("button", { name: "Confirm and submit" }));
    await screen.findByText("Published version 1");
    expect(refusal()).toBe(""); expect(result()).toBe("published");
  });
  it("does not carry a previous refusal into a new indeterminate attempt", async () => {
    mocks.retry.mockResolvedValue(fresh("other"));
    await submit(); await screen.findByText(/Context changed/);
    expect(refusal()).toBe("CONTEXT_UNAVAILABLE");
    mocks.retry.mockResolvedValue(fresh());
    mocks.capture.mockRejectedValue(new WorkbenchContextError(503, "OUTCOME_UNKNOWN", "", []));
    fireEvent.click(screen.getByRole("button", { name: "Confirm and submit" }));
    await screen.findByText(/Outcome is unknown/);
    expect(refusal()).toBe(""); expect(result()).toBe("");
  });
  it("adds no visible text, so the page a person reads is unchanged", async () => {
    await submit(); await screen.findByText("Published version 1");
    expect(node("[data-batch-submit-result]")).not.toBeVisible();
    expect(node("[data-batch-submit-refusal]")).not.toBeVisible();
  });
});
