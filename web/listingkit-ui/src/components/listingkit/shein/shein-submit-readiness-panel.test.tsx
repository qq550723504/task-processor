import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { SheinSubmitReadinessPanel } from "@/components/listingkit/shein/shein-submit-readiness-panel";

describe("SheinSubmitReadinessPanel", () => {
  it("renders blocking items and executable category repair entry", async () => {
    const user = userEvent.setup();
    const onSelectBlockingItem = vi.fn(() => true);
    const onRunPrimaryAction = vi.fn();

    render(
      <SheinSubmitReadinessPanel
        readiness={{
          status: "blocked",
          summary: ["当前仍有关键字段未完成"],
          blocking_items: [
            {
              key: "category",
              label: "类目骨架",
              message: "类目、类目层级和 product_type_id 需要确认",
              suggested_action: "确认类目",
              field_paths: ["shein.category_id"],
              reason: {
                summary: "当前商品还没有确认到可提交的 SHEIN 类目骨架。",
              },
            },
          ],
          warning_items: [
            {
              key: "manual_notes",
              label: "人工备注",
              message: "仍有人工备注未处理",
              suggested_action: "处理备注",
            },
          ],
        }}
        checklist={{
          required: [{ key: "category", label: "类目骨架", status: "blocking" }],
          recommended: [{ key: "request_draft", label: "请求草稿", status: "ready" }],
        }}
        workspaceOverview={{
          headline: "SHEIN 工作台待修复",
          subheadline: "当前仍有关键字段未完成",
          primary_action: "确认类目",
          primary_action_key: "category",
          highlights: ["类目待处理"],
          next_actions: ["确认类目", "确认属性"],
          submit_state: {
            status: "blocked",
            blocking_count: 1,
            warning_count: 1,
          },
        }}
        canSelectBlockingItem={(item) => item.key === "category"}
        onSelectBlockingItem={onSelectBlockingItem}
        canRunPrimaryAction={(key) => key === "category"}
        onRunPrimaryAction={onRunPrimaryAction}
      />,
    );

    expect(screen.getByText("SHEIN publish readiness")).toBeInTheDocument();
    expect(screen.getByText("Blocked")).toBeInTheDocument();
    expect(screen.getAllByText("类目骨架")).toHaveLength(2);
    expect(screen.getByText("当前商品还没有确认到可提交的 SHEIN 类目骨架。")).toBeInTheDocument();
    expect(screen.getByText("Required")).toBeInTheDocument();

    const buttons = screen.getAllByRole("button", { name: "Open fix path" });
    await user.click(buttons[1]);
    expect(onSelectBlockingItem).toHaveBeenCalledTimes(1);
    expect(onSelectBlockingItem.mock.calls[0][0]).toMatchObject({ key: "category" });

    await user.click(buttons[0]);
    expect(onRunPrimaryAction).toHaveBeenCalled();
  });

  it("renders warnings without repair button when no executable path exists", () => {
    render(
      <SheinSubmitReadinessPanel
        readiness={{
          status: "ready_with_warnings",
          warning_items: [
            {
              key: "manual_notes",
              label: "人工备注",
              message: "仍有人工备注未处理",
              suggested_action: "处理备注",
            },
          ],
        }}
        workspaceOverview={{
          submit_state: {
            status: "ready_with_warnings",
            warning_count: 1,
          },
        }}
      />,
    );

    expect(screen.getByText("Ready with warnings")).toBeInTheDocument();
    expect(screen.getByText("人工备注")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Open fix path" })).not.toBeInTheDocument();
  });

  it("renders ready state without blocker lists", () => {
    const onSubmit = vi.fn();
    const onSaveDraft = vi.fn();
    render(
      <SheinSubmitReadinessPanel
        readiness={{
          status: "ready",
          summary: ["SHEIN 资料包已具备提交前所需的关键骨架"],
        }}
        submission={{
          last_action: "publish",
          last_status: "success",
          last_result: {
            message: "success",
          },
        }}
        workspaceOverview={{
          headline: "SHEIN 工作台已就绪",
          primary_action: "Submit to SHEIN",
          primary_action_key: "submit",
          next_actions: ["提交到 SHEIN"],
          submit_state: {
            status: "ready",
            ready: true,
          },
        }}
        canSubmit
        onSubmit={onSubmit}
        onSaveDraft={onSaveDraft}
      />,
    );

    expect(screen.getByText("Ready")).toBeInTheDocument();
    expect(screen.getByText("SHEIN 工作台已就绪")).toBeInTheDocument();
    expect(screen.getAllByText("Submit to SHEIN")).toHaveLength(1);
    expect(screen.getByText("Save draft or submit to SHEIN")).toBeInTheDocument();
    expect(screen.getByText("Latest submission")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save to SHEIN draft" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Submit to SHEIN" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Open fix path" })).not.toBeInTheDocument();
    expect(screen.queryByText("Blocking items")).not.toBeInTheDocument();
  });

  it("renders current submit attempt status and error", () => {
    const { rerender } = render(
      <SheinSubmitReadinessPanel
        readiness={{
          status: "ready",
        }}
        canSubmit
        isSubmitting
        submitAction="save_draft"
        onSubmit={vi.fn()}
        onSaveDraft={vi.fn()}
      />,
    );

    expect(screen.getByText("Current submit attempt")).toBeInTheDocument();
    expect(screen.getByText("Saving to SHEIN draft...")).toBeInTheDocument();

    rerender(
      <SheinSubmitReadinessPanel
        readiness={{
          status: "ready",
        }}
        canSubmit
        submitAction="save_draft"
        submitErrorMessage="SHEIN image upload unavailable: token missing"
        onSubmit={vi.fn()}
        onSaveDraft={vi.fn()}
      />,
    );

    expect(screen.getByText("Save draft failed")).toBeInTheDocument();
    expect(
      screen.getByText("SHEIN image upload unavailable: token missing"),
    ).toBeInTheDocument();
  });

  it("keeps blocker repair entries visible in compact workspace mode", async () => {
    const user = userEvent.setup();
    const onSelectBlockingItem = vi.fn();

    render(
      <SheinSubmitReadinessPanel
        compact
        readiness={{
          status: "blocked",
          blocking_items: [
            {
              key: "attribute_review",
              label: "属性复核",
              message: "普通属性仍需要人工确认",
              suggested_action: "确认属性",
            },
          ],
        }}
        workspaceOverview={{
          primary_action: "确认属性",
          primary_action_key: "attribute_review",
          submit_state: {
            status: "blocked",
            blocking_count: 1,
          },
        }}
        canSelectBlockingItem={(item) => item.key === "attribute_review"}
        onSelectBlockingItem={onSelectBlockingItem}
      />,
    );

    expect(screen.getByText("Blocking items")).toBeInTheDocument();
    expect(screen.getByText("属性复核")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Open fix path" }));
    expect(onSelectBlockingItem).toHaveBeenCalledWith(
      expect.objectContaining({ key: "attribute_review" }),
    );
  });
});
