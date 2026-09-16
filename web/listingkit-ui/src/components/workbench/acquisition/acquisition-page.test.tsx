import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { AcquisitionAPIError, type AcquisitionOperation } from "@/lib/api/product-acquisition";
import { AcquisitionPage } from "./acquisition-page";

const calls = vi.hoisted(() => ({ acquire: vi.fn(), verify: vi.fn(), read: vi.fn(), readProduct: vi.fn(), context: {} as Record<string, unknown>, push: vi.fn() }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => calls.context }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: calls.push }) }));
vi.mock("@/lib/api/product-acquisition", async original => ({ ...await original<object>(), acquire1688: calls.acquire, verify1688: calls.verify, readAcquisition: calls.read, readAcquisitionProduct: calls.readProduct }));

const operationID = "11111111-1111-4111-8111-111111111111";
function deferred<T>() { let resolve!: (value: T) => void, reject!: (reason?: unknown) => void; const promise = new Promise<T>((ok, fail) => { resolve = ok; reject = fail; }); return { promise, resolve, reject }; }
function tree() { return <AcquisitionPage />; }

beforeEach(() => {
  calls.context = { user: { id: "actor-A" }, effectiveOrganization: { id: "organization-A" } };
  calls.acquire.mockReset(); calls.verify.mockReset(); calls.read.mockReset(); calls.readProduct.mockReset(); calls.push.mockReset();
});
afterEach(cleanup);

it("aborts an A-scope submission and ignores its late published result after switching to B", async () => {
  const late = deferred<{ outcome: "published"; operationId: string; productKey: string }>();
  calls.acquire.mockReturnValue(late.promise);
  const view = render(tree());
  await userEvent.type(screen.getByLabelText("1688 商品页或 offer ID"), "https://detail.1688.com/offer/123.html");
  await userEvent.click(screen.getByRole("button", { name: "提交采集" }));
  await waitFor(() => expect(calls.acquire).toHaveBeenCalledOnce());
  const signal = calls.acquire.mock.calls[0][1] as AbortSignal;

  calls.context = { user: { id: "actor-B" }, effectiveOrganization: { id: "organization-B" } };
  view.rerender(tree());
  expect(signal.aborted).toBe(true);
  await act(async () => late.resolve({ outcome: "published", operationId: operationID, productKey: "crawler:1688:123" }));

  expect(calls.push).not.toHaveBeenCalled();
  expect(screen.queryByText(`操作 ${operationID}`)).not.toBeInTheDocument();
});

it("aborts an A-scope recovery and ignores its late published result after switching to B", async () => {
  const late = deferred<{ outcome: "published"; operationId: string; productKey: string }>();
  calls.read.mockReturnValue(late.promise);
  const view = render(tree());
  await userEvent.type(screen.getByLabelText("操作 ID"), operationID);
  await userEvent.click(screen.getByRole("button", { name: "读取" }));
  await waitFor(() => expect(calls.read).toHaveBeenCalledOnce());
  const signal = calls.read.mock.calls[0][2] as AbortSignal;

  calls.context = { user: { id: "actor-B" }, effectiveOrganization: { id: "organization-B" } };
  view.rerender(tree());
  expect(signal.aborted).toBe(true);
  await act(async () => late.resolve({ outcome: "published", operationId: operationID, productKey: "crawler:1688:123" }));

  expect(calls.push).not.toHaveBeenCalled();
  expect(screen.queryByText(`操作 ${operationID}`)).not.toBeInTheDocument();
});

it("keeps the original key when an uncertain submission is explicitly verified", async () => {
  calls.acquire.mockRejectedValueOnce(new AcquisitionAPIError("OUTCOME_UNKNOWN", 503));
  calls.verify.mockResolvedValueOnce({ outcome: "prepared", operationId: operationID, productKey: "crawler:1688:123" });
  render(tree());
  await userEvent.type(screen.getByLabelText("1688 商品页或 offer ID"), "https://detail.1688.com/offer/123.html");
  await userEvent.click(screen.getByRole("button", { name: "提交采集" }));
  await screen.findByRole("alert");
  const original = calls.acquire.mock.calls[0][0] as AcquisitionOperation;
  await userEvent.click(screen.getByRole("button", { name: "使用原 key 核实" }));
  await waitFor(() => expect(calls.verify).toHaveBeenCalledOnce());
  expect(calls.verify.mock.calls[0][0]).toEqual(original);
});
