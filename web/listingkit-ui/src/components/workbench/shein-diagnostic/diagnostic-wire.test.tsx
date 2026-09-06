import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { diagnosticFixture } from "@/test/fixtures/shein-diagnostic";
import { SheinDiagnosticPage } from "./diagnostic-page";

vi.mock("@/components/providers/workbench-context-provider", () => ({
  useWorkbenchContext: () => ({ user: { id: "reader" }, effectiveOrganization: { id: "200", name: "受控企业" }, roles: [], retry: vi.fn() }),
}));
let client: QueryClient;
beforeEach(() => { client = new QueryClient(); });
afterEach(() => { cleanup(); client.clear(); vi.unstubAllGlobals(); });
function show(payload: unknown, status = 200) {
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async () => Response.json(payload, { status })));
  render(<QueryClientProvider client={client}><SheinDiagnosticPage recordId="c78285de-a1a8-4cc5-8aae-93ad6851da11" /></QueryClientProvider>);
}

describe("page → actual #323 client/decoder (synthetic HTTP responses)", () => {
  it("renders the valid wire report", async () => {
    show(diagnosticFixture());
    expect(await screen.findByText("发现需要处理的问题")).toBeVisible();
  });
  it.each([
    ["null blockers", () => ({ ...diagnosticFixture(), offline_checks: { ...diagnosticFixture().offline_checks, blockers: null } })],
    ["missing read time", () => ({ ...diagnosticFixture(), input: { ...diagnosticFixture().input, read_at: undefined } })],
    ["missing digest", () => ({ ...diagnosticFixture(), input: { ...diagnosticFixture().input, actual_digest: undefined } })],
    ["wrong action", () => diagnosticFixture("save_draft")],
    ["invalid report", () => ({ blockers: [] })],
  ])("fails closed for %s", async (_name, payload) => {
    show(payload());
    expect(await screen.findByRole("alert")).toHaveTextContent("诊断响应不合法");
    expect(screen.queryByText("发现需要处理的问题")).not.toBeInTheDocument();
    expect(screen.queryByText(/离线项通过/)).not.toBeInTheDocument();
  });
  it("uses actual typed authorization error without exposing the raw message", async () => {
    show({ code: "PERMISSION_DENIED", message: "private upstream detail", requestId: "test", fieldErrors: [] }, 403);
    expect(await screen.findByRole("alert")).toHaveTextContent("没有读取这份资料的权限");
    expect(screen.queryByText("private upstream detail")).not.toBeInTheDocument();
  });
});
