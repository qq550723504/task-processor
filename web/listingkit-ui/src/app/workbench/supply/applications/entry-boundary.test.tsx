import {afterEach, expect, it, vi} from "vitest";
import ProgressPage from "./progress/page";
import RecordsPage from "./records/page";

const gates = vi.hoisted(() => ({connection:vi.fn(), notFound:vi.fn(() => {throw new Error("NOT_FOUND")})}));
vi.mock("next/server", () => ({connection:gates.connection}));
vi.mock("next/navigation", () => ({notFound:gates.notFound}));
const original = process.env.LISTINGKIT_SUPPLY_MARKET_ENABLED;
afterEach(() => {if(original === undefined) delete process.env.LISTINGKIT_SUPPLY_MARKET_ENABLED; else process.env.LISTINGKIT_SUPPLY_MARKET_ENABLED=original; vi.clearAllMocks()});
it.each([ProgressPage, RecordsPage])("keeps a new application leaf closed without its installed runtime (%#)", async Page => {
  process.env.LISTINGKIT_SUPPLY_MARKET_ENABLED="false";
  await expect(Page()).rejects.toThrow("NOT_FOUND");
  expect(gates.connection).toHaveBeenCalledOnce();
});
it.each([[ProgressPage,"progress"], [RecordsPage,"records"]] as const)("connects the installed runtime to its actual consumer (%s)", async (Page, view) => {
  process.env.LISTINGKIT_SUPPLY_MARKET_ENABLED="true";
  const element=await Page();
  expect(element.props.view).toBe(view);
  expect(gates.notFound).not.toHaveBeenCalled();
});
