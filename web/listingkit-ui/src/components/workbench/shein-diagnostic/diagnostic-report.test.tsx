import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";

import { diagnosticFixture } from "@/test/fixtures/shein-diagnostic";
import { DiagnosticReport } from "./diagnostic-report";

afterEach(cleanup);

describe("SHEIN diagnostic report (synthetic contract fixture)", () => {
  it("separates blockers, warnings, checks and unassessed scope and expands literal details", async () => {
    const user = userEvent.setup();
    const report = diagnosticFixture();
    report.offline_checks.blockers[0].guidance = "<img src=x onerror=alert(1)>";
    const { container } = render(<DiagnosticReport report={report} recordId="c78285de-a1a8-4cc5-8aae-93ad6851da11" />);
    const problems = screen.getByRole("region", { name: "问题（1）" });
    expect(within(problems).getByText("缺少商品名称")).toBeVisible();
    await user.click(within(problems).getByText("缺少商品名称"));
    expect(within(problems).getByText("product.name")).toBeVisible();
    expect(within(problems).getByText("<img src=x onerror=alert(1)>")).toBeVisible();
    expect(container.querySelector("img")).toBeNull();
    expect(screen.getByRole("region", { name: "警告（1）" })).toBeVisible();
    expect(screen.getByRole("region", { name: "检查项目（2）" })).toBeVisible();
    expect(screen.getByRole("heading", { name: "未评估范围" })).toBeVisible();
    expect(screen.getByText("no_authoritative_package_freshness")).toBeInTheDocument();
    expect(screen.queryByText(/^可发布$|全部检查通过/)).not.toBeInTheDocument();
  });

  it("never equates offline ready with release permission and preserves binding evidence", async () => {
    const report = { ...diagnosticFixture("save_draft"), offline_checks: { status: "ready" as const, checks: [], blockers: [], warnings: [] } };
    render(<DiagnosticReport report={report} recordId="c78285de-a1a8-4cc5-8aae-93ad6851da11" />);
    expect(screen.getByText("离线项通过，仍有未评估范围")).toBeVisible();
    expect(screen.getByText("按本地草稿规则检查")).toBeVisible();
    await userEvent.click(screen.getByText("检查技术信息"));
    expect(screen.getByText(report.input.actual_digest)).toBeVisible();
    expect(screen.getByText(report.input.binding_version)).toBeVisible();
    expect(screen.getByText(report.input.read_at)).toBeVisible();
    expect(screen.queryByRole("button", { name: /发布|Apply|保存草稿/ })).not.toBeInTheDocument();
  });
});
