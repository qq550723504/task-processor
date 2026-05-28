import { describe, expect, it } from "vitest";

import {
  buildGroupedSDSBaselineActionLabel,
  buildGroupedSDSBaselineHandoff,
  buildGroupedSDSBaselineHelperText,
  buildRecentBatchBaselineAlert,
  getSDSBaselineStatusBadgeVariant,
  getSDSBaselineStatusLabel,
} from "@/lib/shein-studio/sds-baseline-ui";

describe("sds baseline ui helpers", () => {
  it("maps baseline statuses to consistent badge labels and variants", () => {
    expect(getSDSBaselineStatusLabel("ready")).toBe("Baseline 已就绪");
    expect(getSDSBaselineStatusBadgeVariant("ready")).toBe("success");

    expect(getSDSBaselineStatusLabel("baseline_cached")).toBe("Baseline 待校验");
    expect(getSDSBaselineStatusBadgeVariant("baseline_cached")).toBe("warning");

    expect(getSDSBaselineStatusLabel("blocked")).toBe("Baseline 已阻断");
    expect(getSDSBaselineStatusBadgeVariant("blocked")).toBe("danger");

    expect(getSDSBaselineStatusLabel("failed")).toBe("Baseline 异常");
    expect(getSDSBaselineStatusBadgeVariant("failed")).toBe("danger");

    expect(getSDSBaselineStatusLabel("missing")).toBe("Baseline 缺失");
    expect(getSDSBaselineStatusBadgeVariant("missing")).toBe("warning");

    expect(getSDSBaselineStatusLabel("loading")).toBe("Baseline 检查中");
    expect(getSDSBaselineStatusBadgeVariant("loading")).toBe("neutral");
  });

  it("builds grouped helper copy from status and reason", () => {
    expect(
      buildGroupedSDSBaselineHelperText({
        status: "baseline_cached",
        reason: "",
      }),
    ).toBe("这款商品只完成了 baseline 缓存，暂时还不能加入 grouped 批量上品。");

    expect(
      buildGroupedSDSBaselineHelperText({
        status: "blocked",
        reasonCode: "layer_missing",
        reason: "",
      }),
    ).toBe("这款商品的 baseline 选中图层在 SDS 设计面里不存在。");
  });

  it("builds grouped action labels from status", () => {
    expect(buildGroupedSDSBaselineActionLabel({ status: "missing", reason: "" })).toBe(
      "回选并预热",
    );
    expect(
      buildGroupedSDSBaselineActionLabel({
        status: "baseline_cached",
        reason: "",
      }),
    ).toBe("回选并继续校验");
    expect(buildGroupedSDSBaselineActionLabel({ status: "blocked", reason: "" })).toBe(
      "回选并修复",
    );
    expect(buildGroupedSDSBaselineActionLabel({ status: "loading", reason: "" })).toBe(
      "回选并等待",
    );
  });

  it("builds grouped handoff guidance from status", () => {
    expect(
      buildGroupedSDSBaselineHandoff({
        status: "missing",
        reason: "",
      }),
    ).toEqual({
      action: "warm_baseline",
      actionLabel: "一键预热并校验 baseline",
      message:
        "这款候选商品还没有 baseline 缓存。先在当前工作台完成一次生成或预热，再回来继续校验并加入 grouped 批量上品。",
    });

    expect(
      buildGroupedSDSBaselineHandoff({
        status: "baseline_cached",
        reason: "",
      }),
    ).toEqual({
      action: "warm_baseline",
      actionLabel: "继续 baseline 校验",
      message:
        "这款候选商品已经完成 baseline 缓存，但还没通过进一步校验。先继续校验，再回来加入 grouped 批量上品。",
    });

    expect(
      buildGroupedSDSBaselineHandoff({
        status: "blocked",
        reasonCode: "login_missing_credentials",
        reason: "",
      }),
    ).toEqual({
      action: "open_sds_login",
      actionLabel: "去处理 SDS 登录",
      message: "当前 SDS 登录缺少 access token。",
    });

    expect(
      buildGroupedSDSBaselineHandoff({
        status: "blocked",
        reason: "SDS login state is missing access token.",
      }),
    ).toEqual({
      action: "open_sds_login",
      actionLabel: "去处理 SDS 登录",
      message: "SDS login state is missing access token.",
    });

    expect(
      buildGroupedSDSBaselineHandoff({
        status: "blocked",
        reason: "",
      }),
    ).toEqual({
      action: "focus_generate",
      actionLabel: "去生成区修复",
      message:
        "这款候选商品的 baseline 校验已被阻断。请先修复模板、图层或 SDS 登录态问题，再尝试加入 grouped 批量上品。",
    });

    expect(
      buildGroupedSDSBaselineHandoff({
        status: "loading",
        reason: "",
      }),
    ).toEqual({
      action: "focus_generate",
      actionLabel: "去生成区查看",
      message:
        "这款候选商品的 baseline 状态还在检查中。稍等片刻，确认校验结果后再加入 grouped 批量上品。",
    });
  });

  it("builds recent batch baseline alerts from status", () => {
    expect(
      buildRecentBatchBaselineAlert({
        status: "baseline_cached",
        reason: "",
      }),
    ).toEqual({
      tone: "danger",
      label: "Baseline 待校验",
      detail: "组内仍有商品只完成了 baseline 缓存，尚未通过进一步校验。",
    });

    expect(
      buildRecentBatchBaselineAlert({
        status: "blocked",
        reasonCode: "prototype_group_mismatch",
        reason: "",
      }),
    ).toEqual({
      tone: "danger",
      label: "Baseline 校验未通过",
      reasonCode: "prototype_group_mismatch",
      detail: "这款商品的 prototype group 与当前 SDS 设计面不匹配。",
    });
  });
});
