import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { SheinSubmitReadinessPanel } from "@/components/listingkit/shein/shein-submit-readiness-panel";

describe("SheinSubmitReadinessPanel", () => {
  it("renders blocking items and executable category repair entry", async () => {
    const user = userEvent.setup();
    const onSelectBlockingItem = vi.fn((item: { key?: string }) => item);
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

    expect(screen.getByText("SHEIN 发布检查")).toBeInTheDocument();
    expect(screen.getByText("有阻断")).toBeInTheDocument();
    expect(screen.getByText("待处理问题")).toBeInTheDocument();
    expect(screen.getAllByText("类目骨架")).toHaveLength(2);
    expect(screen.getByText("当前商品还没有确认到可提交的 SHEIN 类目骨架。")).toBeInTheDocument();
    expect(screen.getByText("必须完成")).toBeInTheDocument();

    await user.click(screen.getAllByRole("button", { name: "去确认类目" })[0]);
    expect(onSelectBlockingItem).toHaveBeenCalledTimes(1);
    expect(onSelectBlockingItem.mock.calls[0][0]).toMatchObject({ key: "category" });

    await user.click(screen.getAllByRole("button", { name: "去处理" })[0]);
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

    expect(screen.getByText("可提交但有提醒")).toBeInTheDocument();
    expect(screen.getByText("待处理问题")).toBeInTheDocument();
    expect(screen.getByText("存在未识别的问题")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "去处理" })).not.toBeInTheDocument();
  });

  it("renders pod platform blockers with explicit pod badge and repair action", async () => {
    const user = userEvent.setup();
    const onSelectBlockingItem = vi.fn();

    render(
      <SheinSubmitReadinessPanel
        readiness={{
          status: "blocked",
          blocking_items: [
            {
              key: "pod_platform",
              label: "POD 平台处理",
              message: "SDS 平台处理为发布前置，当前不可提交：design template unavailable",
              suggested_action: "处理 POD 平台结果",
            },
          ],
        }}
        workspaceOverview={{
          submit_state: {
            status: "blocked",
            blocking_count: 1,
          },
        }}
        canSelectBlockingItem={(item) => item.key === "pod_platform"}
        onSelectBlockingItem={onSelectBlockingItem}
      />,
    );

    expect(screen.getByText("POD 平台处理")).toBeInTheDocument();
    expect(screen.getByText("POD 平台")).toBeInTheDocument();

    await user.click(screen.getAllByRole("button", { name: "去检查 POD 结果" })[0]);
    expect(onSelectBlockingItem).toHaveBeenCalledWith(
      expect.objectContaining({ key: "pod_platform" }),
    );
  });

  it("renders pod size-image degradation warnings with explicit size-image wording", () => {
    render(
      <SheinSubmitReadinessPanel
        readiness={{
          status: "ready_with_warnings",
          warning_items: [
            {
              key: "pod_platform",
              label: "POD 平台处理",
              message:
                "SDS 平台处理失败，当前将按降级素材继续发布：size image render unavailable",
              suggested_action: "处理 POD 平台结果",
            },
          ],
        }}
        workspaceOverview={{
          submit_state: {
            status: "ready_with_warnings",
            warning_count: 1,
          },
        }}
      />
    );

    expect(screen.getByText("POD 尺寸图降级")).toBeInTheDocument();
    expect(screen.getByText("POD 尺寸图")).toBeInTheDocument();
  });

  it("renders freshness blockers with mapped labels instead of integration-gap fallback", async () => {
    const user = userEvent.setup();
    const onSelectBlockingItem = vi.fn();

    render(
      <SheinSubmitReadinessPanel
        readiness={{
          status: "blocked",
          blocking_items: [
            {
              key: "shein_online_auth",
              label: "SHEIN 在线登录态",
              message: "SHEIN 提交店铺当前不可用，请先刷新登录态后再提交：store token missing",
              suggested_action: "重新登录 SHEIN 店铺",
            },
            {
              key: "shein_category_template_freshness",
              label: "类目模板新鲜度",
              message: "当前类目模板已发生变化",
              suggested_action: "刷新类目模板",
            },
          ],
        }}
        workspaceOverview={{
          submit_state: {
            status: "blocked",
            blocking_count: 2,
          },
        }}
        canSelectBlockingItem={() => true}
        onSelectBlockingItem={onSelectBlockingItem}
      />,
    );

    expect(screen.getByText("SHEIN 在线登录态")).toBeInTheDocument();
    expect(screen.getByText("店铺登录")).toBeInTheDocument();
    expect(screen.getByText("类目模板新鲜度")).toBeInTheDocument();
    expect(screen.getByText("类目模板")).toBeInTheDocument();
    expect(screen.queryByText("未支持自动跳转")).not.toBeInTheDocument();

    await user.click(screen.getAllByRole("button", { name: "去登录店铺" })[0]);
    expect(onSelectBlockingItem).toHaveBeenCalled();
  });

  it("surfaces unknown blocker keys as integration gaps", () => {
    render(
      <SheinSubmitReadinessPanel
        readiness={{
          status: "blocked",
          blocking_items: [
            {
              key: "remote_compliance_hold",
              label: "平台合规拦截",
              message: "SHEIN 返回了新的阻断类型",
              suggested_action: "联系工程确认映射",
            },
          ],
        }}
        canSelectBlockingItem={() => false}
      />,
    );

    expect(screen.getByText("平台合规拦截")).toBeInTheDocument();
    expect(screen.getByText("未支持自动跳转")).toBeInTheDocument();
    expect(screen.getByText("原始 key：remote_compliance_hold")).toBeInTheDocument();
    expect(
      screen.getByText("请记录这个阻断项 key，并补充 SHEIN readiness 映射后再验收。"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "去处理" })).not.toBeInTheDocument();
  });

  it("renders ready state without blocker lists", () => {
    const onSubmit = vi.fn();
    const onSaveDraft = vi.fn();
    const { container } = render(
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
            success: true,
            spu_name: "h2605201253579421",
          },
        }}
        workspaceOverview={{
          headline: "SHEIN 工作台已就绪",
          primary_action: "提交到 SHEIN",
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

    expect(screen.getByText("可提交")).toBeInTheDocument();
    expect(screen.getByText("SHEIN 工作台已就绪")).toBeInTheDocument();
    expect(screen.getAllByText("提交到 SHEIN")).toHaveLength(1);
    expect(screen.getByText("保存草稿或提交到 SHEIN")).toBeInTheDocument();
    expect(screen.getByText("最新提交记录")).toBeInTheDocument();
    expect(screen.getByText("已提交到 SHEIN")).toBeInTheDocument();
    expect(
      screen.getByText("商品资料已提交到 SHEIN 发布接口，请以 SHEIN 后台最终状态为准。"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存到 SHEIN 草稿箱" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "已发布到 SHEIN" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "保存到 SHEIN 草稿箱" })).toHaveClass("w-full");
    expect(container.querySelector(".sm\\:w-auto")).not.toBeNull();
    expect(screen.queryByRole("button", { name: "去处理" })).not.toBeInTheDocument();
    expect(screen.queryByText("阻断项")).not.toBeInTheDocument();
  });

  it("renders save draft success as a customer-facing Chinese result", () => {
    render(
      <SheinSubmitReadinessPanel
        readiness={{ status: "ready" }}
        submission={{
          last_action: "save_draft",
          last_status: "success",
          last_result: {
            message: "OK",
            success: true,
          },
        }}
      />,
    );

    expect(screen.getByText("最新提交记录")).toBeInTheDocument();
    expect(screen.getByText("已保存到 SHEIN 草稿箱")).toBeInTheDocument();
    expect(
      screen.getByText("商品资料已进入 SHEIN 草稿箱，不会直接上架。"),
    ).toBeInTheDocument();
  });

  it("renders publish failure with collapsed raw submission details", () => {
    render(
      <SheinSubmitReadinessPanel
        readiness={{ status: "blocked" }}
        submission={{
          last_action: "publish",
          last_status: "failed",
          last_error: "raw SHEIN response: square image missing",
          last_result: {
            success: false,
            message: "validation failed",
            validation_notes: ["方形图必须有一个"],
          },
        }}
      />,
    );

    expect(screen.getByText("发布失败")).toBeInTheDocument();
    expect(screen.getByText("请根据待处理问题修复后再次提交。")).toBeInTheDocument();
    expect(screen.getByText("查看原始接口返回")).toBeInTheDocument();
    expect(screen.getByText("查看原始 SHEIN 校验提示")).toBeInTheDocument();
    expect(screen.getAllByText("raw SHEIN response: square image missing").length).toBeGreaterThan(0);
  });

  it("renders failed submission phase, reason, recoverability, and next action", () => {
    render(
      <SheinSubmitReadinessPanel
        readiness={{ status: "blocked" }}
        submission={{
          last_action: "publish",
          last_status: "failed",
          last_error: "remote validation rejected size image",
          publish: {
            action: "publish",
            status: "failed",
            phase: "pre_validate",
            error: "方形图必须有一个",
            request_id: "submit-failed-123",
            recoverable: true,
            next_action: "补齐方形图后重新发布",
          },
        }}
      />,
    );

    expect(screen.getByText("失败阶段：SHEIN 预校验")).toBeInTheDocument();
    expect(screen.getByText("失败原因：方形图必须有一个")).toBeInTheDocument();
    expect(screen.getByText("是否可重试：可由运营修复后重试")).toBeInTheDocument();
    expect(screen.getByText("下一步：补齐方形图后重新发布")).toBeInTheDocument();
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

    expect(screen.getByText("当前提交")).toBeInTheDocument();
    expect(screen.getByText("正在保存到 SHEIN 草稿箱...")).toBeInTheDocument();

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

    expect(screen.getByText("保存草稿失败")).toBeInTheDocument();
    expect(
      screen.getByText("SHEIN image upload unavailable: token missing"),
    ).toBeInTheDocument();
    expect(screen.getByText("发生了什么")).toBeInTheDocument();
    expect(screen.getByText("可能影响")).toBeInTheDocument();
    expect(screen.getByText("下一步怎么做")).toBeInTheDocument();
    expect(
      screen.getByText("本次不会把资料保存到 SHEIN 草稿箱，请先处理图片上传或阻断项后再重试。"),
    ).toBeInTheDocument();
  });

  it("renders backend submit phase when an attempt is in flight", () => {
    render(
      <SheinSubmitReadinessPanel
        readiness={{
          status: "ready",
        }}
        submission={{
          current_action: "publish",
          current_phase: "upload_images",
          current_request_id: "submit-123",
        }}
        canSubmit
        onSubmit={vi.fn()}
        onSaveDraft={vi.fn()}
      />,
    );

    expect(screen.getByText("当前提交")).toBeInTheDocument();
    expect(screen.getByText("正在发布到 SHEIN")).toBeInTheDocument();
    expect(screen.getByText("当前阶段：上传图片")).toBeInTheDocument();
  });

  it("keeps blocker repair entries visible in compact workspace mode", async () => {
    const user = userEvent.setup();
    const onSelectBlockingItem = vi.fn();
    const onRunPrimaryAction = vi.fn();

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
        canSelectBlockingItem={(item) => item.key === "attributes"}
        onSelectBlockingItem={onSelectBlockingItem}
        canRunPrimaryAction={(key) => key === "attribute_review"}
        onRunPrimaryAction={onRunPrimaryAction}
      />,
    );

    expect(screen.queryByText("待处理问题")).not.toBeInTheDocument();
    expect(screen.queryByText("商品属性需要补齐")).not.toBeInTheDocument();
    expect(screen.getByText("下一步处理")).toBeInTheDocument();
    expect(screen.getByText("确认属性")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "去处理" }));
    expect(onRunPrimaryAction).toHaveBeenCalledWith("attribute_review");
  });
});
