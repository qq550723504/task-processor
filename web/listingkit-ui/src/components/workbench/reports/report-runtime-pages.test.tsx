import { render, screen, cleanup } from "@testing-library/react";
import { Fragment } from "react";
import { afterEach, expect, it, vi } from "vitest";
import Page from "@/app/workbench/ai/reports/page";
import ViewPage from "@/app/workbench/ai/reports/[view]/page";
import { ReportPage } from "./report-page";

vi.mock("./report-page", () => ({ ReportPage: vi.fn(({ view }: { view?: string }) => <div data-testid="report-business">{view ?? "overview"}</div>) }));
afterEach(() => { cleanup(); vi.unstubAllEnvs(); vi.clearAllMocks(); });

it.each([undefined, "false"])("direct report routes show unavailable without runtime configuration (%s)", async enabled => {
  vi.stubEnv("LISTINGKIT_REPORT_CENTER_ENABLED", enabled);
  const views = await Promise.all(["recent", "favorites", "all"].map(view => ViewPage({ params: Promise.resolve({ view }) })));
  render(<><Page />{views.map((element, index) => <Fragment key={index}>{element}</Fragment>)}</>);
  expect(screen.getAllByText("我的报告尚未启用")).toHaveLength(4);
  expect(screen.queryByTestId("report-business")).not.toBeInTheDocument();
  expect(ReportPage).not.toHaveBeenCalled();
});
it("configured direct report routes keep the existing business pages", async () => {
  vi.stubEnv("LISTINGKIT_REPORT_CENTER_ENABLED", "true");
  const views = await Promise.all(["recent", "favorites", "all"].map(view => ViewPage({ params: Promise.resolve({ view }) })));
  render(<><Page />{views.map((element, index) => <Fragment key={index}>{element}</Fragment>)}</>);
  expect(screen.getAllByTestId("report-business")).toHaveLength(4);
  expect(screen.getByText("overview")).toBeInTheDocument();
  expect(screen.getByText("favorites")).toBeInTheDocument();
});
it.each([[undefined, false], ["https://other.test", false], ["https://api.test", true]] as const)("passes same-application Review readiness to all direct report pages (%s)", async (origin, expected) => {
  vi.stubEnv("LISTINGKIT_REPORT_CENTER_ENABLED", "true");
  vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "https://api.test/api/v1");
  vi.stubEnv("PRODUCT_REVIEW_API_ORIGIN", origin);
  const views = await Promise.all(["recent", "favorites", "all"].map(view => ViewPage({ params: Promise.resolve({ view }) })));
  render(<><Page />{views.map((element, index) => <Fragment key={index}>{element}</Fragment>)}</>);
  expect(vi.mocked(ReportPage).mock.calls.map(([props]) => props.titleReviewAvailable)).toEqual([expected, expected, expected, expected]);
});
