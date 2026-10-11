import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import WorkbenchPage from "@/app/workbench/page";
import SupplyPage from "@/app/workbench/supply/page";
import DataPage from "@/app/workbench/data/page";
import AgentsPage from "@/app/workbench/agents/page";
import StorePage from "@/app/workbench/store-center/page";

const fixture = vi.hoisted(() => ({
  redirect: vi.fn(),
  context: { operationsCockpitAvailable: false, permissions: [] as string[],
    isLoading: false, isSwitching: false, selectionRequired: false,
    effectiveOrganization: { id: "org-a" } as { id: string } | null, error: null, blockingError: null },
}));
vi.mock("next/navigation", () => ({ redirect: fixture.redirect }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => fixture.context }));
beforeEach(() => {
  fixture.redirect.mockReset();
  fixture.context.operationsCockpitAvailable = false;
  fixture.context.permissions = [];
  fixture.context.isLoading = fixture.context.isSwitching = fixture.context.selectionRequired = false;
  fixture.context.effectiveOrganization = { id: "org-a" };
});
afterEach(() => { cleanup(); vi.unstubAllEnvs(); });

function expectDestination(page: () => unknown, destination: string) {
  fixture.redirect.mockImplementation(() => { throw new Error("NEXT_REDIRECT"); });
  expect(page).toThrow("NEXT_REDIRECT");
  expect(fixture.redirect).toHaveBeenCalledWith(destination);
}

it.each([
  ["LISTINGKIT_PRODUCT_ACQUISITION_ENABLED", "/workbench/supply/acquisition"],
  ["LISTINGKIT_SUPPLY_MARKET_ENABLED", "/workbench/supply/official"],
  ["LISTINGKIT_SUPPLY_CHAIN_ENABLED", "/workbench/supply/mine"],
])("opens the supplied supply module (%s)", (enabled, destination) => {
  vi.stubEnv("LISTINGKIT_PRODUCT_ACQUISITION_ENABLED", "false");
  vi.stubEnv("LISTINGKIT_SUPPLY_MARKET_ENABLED", "false");
  vi.stubEnv("LISTINGKIT_PRODUCT_COLLECTIONS_ENABLED", "true");
  vi.stubEnv("LISTINGKIT_SUPPLY_CHAIN_ENABLED", "false");
  vi.stubEnv(enabled, "true");
  expectDestination(SupplyPage, destination);
});

it("does not open Supply Chain without its existing Collections dependency", () => {
  vi.stubEnv("LISTINGKIT_PRODUCT_ACQUISITION_ENABLED", "false");
  vi.stubEnv("LISTINGKIT_SUPPLY_MARKET_ENABLED", "false");
  vi.stubEnv("LISTINGKIT_PRODUCT_COLLECTIONS_ENABLED", "false");
  vi.stubEnv("LISTINGKIT_SUPPLY_CHAIN_ENABLED", "true");
  render(<SupplyPage />);
  expect(fixture.redirect).not.toHaveBeenCalled();
  expect(screen.getByText("暂未启用")).toBeVisible();
});

it.each([
  ["true", "true", "/workbench/data/market"],
  ["false", "true", "/workbench/data/mine"],
])("opens only mounted data capabilities (%s/%s)", (data, collections, destination) => {
  vi.stubEnv("LISTINGKIT_DATA_SERVICES_ENABLED", data);
  vi.stubEnv("LISTINGKIT_PRODUCT_COLLECTIONS_ENABLED", collections);
  expectDestination(DataPage, destination);
});

it("keeps missing data modules unavailable", () => {
  vi.stubEnv("LISTINGKIT_DATA_SERVICES_ENABLED", undefined);
  vi.stubEnv("LISTINGKIT_PRODUCT_COLLECTIONS_ENABLED", "false");
  render(<DataPage />);
  expect(fixture.redirect).not.toHaveBeenCalled();
  expect(screen.getByText("暂未启用")).toBeVisible();
});

it("opens the existing Agent configuration catalog without executing an Agent", () => {
  expectDestination(AgentsPage, "/workbench/agents/market");
});
it("opens the existing Store list", () => {
  expectDestination(StorePage, "/workbench/stores");
});

it.each(["goals", "stores", "alerts", "advice"])("opens the granted %s Cockpit page", mode => {
  fixture.context.operationsCockpitAvailable = true;
  fixture.context.permissions = [`workbench.cockpit.${mode}.read`];
  expectDestination(() => render(<WorkbenchPage />), `/workbench/overview/${mode}`);
});
it("uses navigation order when several Cockpit pages are granted", () => {
  fixture.context.operationsCockpitAvailable = true;
  fixture.context.permissions = ["workbench.cockpit.stores.read", "workbench.cockpit.goals.read"];
  expectDestination(() => render(<WorkbenchPage />), "/workbench/overview/goals");
});
it("distinguishes missing Cockpit permission from an unmounted module", () => {
  fixture.context.operationsCockpitAvailable = true;
  render(<WorkbenchPage />);
  expect(fixture.redirect).not.toHaveBeenCalled();
  expect(screen.getByText("没有运营驾驶舱查看权限")).toBeVisible();
  expect(screen.queryByText("经营数据暂未接入")).not.toBeInTheDocument();
});
it("keeps the honest overview when Cockpit is unmounted", () => {
  render(<WorkbenchPage />);
  expect(fixture.redirect).not.toHaveBeenCalled();
  expect(screen.getByText("经营数据暂未接入")).toBeVisible();
});
it.each(["isLoading", "isSwitching", "selectionRequired"] as const)("does not navigate before %s clears", flag => {
  fixture.context.operationsCockpitAvailable = true;
  fixture.context.permissions = ["workbench.cockpit.goals.read"];
  fixture.context[flag] = true;
  render(<WorkbenchPage />);
  expect(fixture.redirect).not.toHaveBeenCalled();
});
