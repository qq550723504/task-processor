import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import Page from "./page";

const navigate = vi.hoisted(() => ({ redirect: vi.fn() }));
vi.mock("next/navigation", () => ({ redirect: navigate.redirect }));
afterEach(() => {
  cleanup();
  navigate.redirect.mockReset();
  vi.unstubAllEnvs();
});

it("opens the existing official tools page when the module is mounted", () => {
  vi.stubEnv("LISTINGKIT_TOOL_MARKET_ENABLED", "true");
  navigate.redirect.mockImplementation(() => { throw new Error("NEXT_REDIRECT"); });
  expect(() => Page()).toThrow("NEXT_REDIRECT");
  expect(navigate.redirect).toHaveBeenCalledWith("/workbench/tools/official");
});

it.each([undefined, "false", "TRUE"])("keeps an unconfigured module unavailable (%s)", (enabled) => {
  vi.stubEnv("LISTINGKIT_TOOL_MARKET_ENABLED", enabled);
  render(<Page />);
  expect(navigate.redirect).not.toHaveBeenCalled();
  expect(screen.getByText("暂未启用")).toBeVisible();
});
