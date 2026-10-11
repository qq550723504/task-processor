import {cleanup, render, screen} from "@testing-library/react";
import {afterEach, beforeEach, expect, it, vi} from "vitest";
import Page from "@/app/workbench/ai/page";

const fixture = vi.hoisted(() => ({
  redirect: vi.fn(), knowledge: false, reports: false, review: false, history: false,
  context: {aiWorkbenchAvailable: false, projectCenterAvailable: false, isLoading: false,
    isSwitching: false, selectionRequired: false, error: null, blockingError: null,
    effectiveOrganization: {id: "org-a"}},
}));
vi.mock("next/navigation", () => ({redirect: fixture.redirect}));
vi.mock("@/components/providers/workbench-context-provider", () => ({useWorkbenchContext: () => fixture.context}));
vi.mock("@/lib/server/knowledge-availability", () => ({isKnowledgeAvailable: () => fixture.knowledge}));
vi.mock("@/lib/server/report-center-availability", () => ({isReportCenterAvailable: () => fixture.reports}));
vi.mock("@/lib/server/product-title-review-request", () => ({configuredProductReviewOrigin: () => fixture.review ? "https://review.test" : null}));
vi.mock("@/lib/server/shein-records-availability", () => ({isSheinRecordsAvailable: () => fixture.history}));
beforeEach(() => {
  fixture.redirect.mockReset();
  fixture.knowledge = fixture.reports = fixture.review = fixture.history = false;
  fixture.context.aiWorkbenchAvailable = fixture.context.projectCenterAvailable = fixture.context.isLoading = fixture.context.isSwitching = false;
  fixture.context.selectionRequired = false;
});
afterEach(cleanup);

it("opens the current project owner without enabling Chat or business tasks", () => {
  fixture.context.projectCenterAvailable = true;
  fixture.knowledge = fixture.reports = true;
  render(<Page />);
  expect(fixture.redirect).toHaveBeenCalledWith("/workbench/ai/projects");
  expect(screen.queryByText("暂未启用")).not.toBeInTheDocument();
});

it.each([
  ["knowledge", "/workbench/ai/knowledge"], ["reports", "/workbench/ai/reports"],
  ["review", "/workbench/ai/tasks/pending/other"], ["history", "/workbench/ai/tasks/completed/history"],
] as const)("opens the independent %s page", (capability, destination) => {
  fixture[capability] = true;
  render(<Page />);
  expect(fixture.redirect).toHaveBeenCalledWith(destination);
});

it("keeps the original Chat entry when its actual runtime is available", () => {
  fixture.context.aiWorkbenchAvailable = fixture.context.projectCenterAvailable = true;
  render(<Page />);
  expect(fixture.redirect).toHaveBeenCalledWith("/workbench/ai/chat");
});

it("opens built-in official reading without optional AI or enterprise knowledge services", () => {
  render(<Page />);
  expect(fixture.redirect).toHaveBeenCalledWith("/workbench/ai/knowledge/official");
});

it.each(["isLoading", "isSwitching", "selectionRequired"] as const)("does not route before %s clears", (flag) => {
  fixture.context.projectCenterAvailable = true;
  fixture.context[flag] = true;
  render(<Page />);
  expect(fixture.redirect).not.toHaveBeenCalled();
});
