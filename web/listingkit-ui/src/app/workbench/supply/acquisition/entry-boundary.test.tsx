import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import AcquisitionEntry from "./page";
import AcquisitionDetailEntry from "./operation/[operation_id]/page";

const state = vi.hoisted(() => ({ available: false }));
vi.mock("next/server", () => ({ connection: vi.fn() }));
vi.mock("next/navigation", () => ({ notFound: () => { throw new Error("NEXT_HTTP_ERROR_FALLBACK;404"); } }));
vi.mock("@/lib/server/product-acquisition-availability", () => ({ isProductAcquisitionAvailable: () => state.available }));
vi.mock("@/components/workbench/acquisition/acquisition-page", () => ({ AcquisitionPage: ({ operationId }: { operationId?: string }) => <p>{operationId ?? "entry"}</p> }));

beforeEach(() => { state.available = false; });
afterEach(cleanup);

it("keeps both direct acquisition URLs closed by default", async () => {
  await expect(AcquisitionEntry()).rejects.toThrow("NEXT_HTTP_ERROR_FALLBACK;404");
  await expect(AcquisitionDetailEntry({ params: Promise.resolve({ operation_id: "11111111-1111-4111-8111-111111111111" }) })).rejects.toThrow("NEXT_HTTP_ERROR_FALLBACK;404");
});

it("opens both URLs only for an explicitly enabled serving deployment", async () => {
  state.available = true;
  render(await AcquisitionEntry());
  expect(screen.getByText("entry")).toBeVisible();
  cleanup();
  render(await AcquisitionDetailEntry({ params: Promise.resolve({ operation_id: "11111111-1111-4111-8111-111111111111" }) }));
  expect(screen.getByText("11111111-1111-4111-8111-111111111111")).toBeVisible();
});
