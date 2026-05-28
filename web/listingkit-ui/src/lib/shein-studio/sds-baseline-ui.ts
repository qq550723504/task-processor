import type { BadgeProps } from "@/components/ui/badge";
import type { SDSBaselineStatus } from "@/lib/types/sds-baseline";
import type { SheinStudioRecentBatchAlert } from "@/lib/types/shein-studio";
import type { SDSGroupedCandidateHandoff } from "@/lib/utils/sds-grouped-candidate-handoff";

export type SDSBaselineDisplayStatus = SDSBaselineStatus | "loading";

export function getSDSBaselineReasonMessage(reasonCode?: string) {
  switch ((reasonCode ?? "").trim()) {
    case "missing_options":
      return "这款商品缺少 baseline 校验参数。";
    case "missing_parent_product":
      return "这款商品缺少 SDS parent product。";
    case "missing_prototype_group":
      return "这款商品缺少 SDS prototype group。";
    case "missing_variant":
      return "这款商品缺少 SDS variant。";
    case "missing_design_type":
      return "这款商品缺少 SDS design type。";
    case "missing_printable_size":
      return "这款商品缺少完整的可印刷尺寸。";
    case "missing_layer":
      return "这款商品缺少 SDS layer 选择。";
    case "login_unavailable":
      return "当前 SDS 登录态不可用。";
    case "login_in_progress":
      return "当前 SDS 登录仍在进行中。";
    case "login_missing_credentials":
      return "当前 SDS 登录缺少 access token。";
    case "login_status_check_failed":
      return "SDS 登录态检查失败。";
    case "product_detail_check_failed":
      return "SDS 商品详情检查失败。";
    case "product_detail_unavailable":
      return "当前 SDS 商品详情不可用。";
    case "design_surface_check_failed":
      return "SDS 设计面检查失败。";
    case "design_surface_unavailable":
      return "当前 SDS 设计面不可用。";
    case "variant_mismatch":
      return "这款商品的 variant 与当前 SDS 设计面不匹配。";
    case "prototype_group_mismatch":
      return "这款商品的 prototype group 与当前 SDS 设计面不匹配。";
    case "layer_missing":
      return "这款商品的 baseline 选中图层在 SDS 设计面里不存在。";
    case "prototype_group_check_failed":
      return "SDS prototype group 检查失败。";
    case "prototype_group_unavailable":
      return "这款商品的 parent product 没有暴露当前选中的 prototype group。";
    case "cache_repository_unavailable":
      return "当前 baseline cache repository 不可用。";
    case "cache_payload_missing":
      return "baseline 缓存条目缺少 canonical payload。";
    case "cache_payload_invalid":
      return "baseline 缓存 payload 已损坏。";
    case "cache_payload_empty":
      return "baseline 缓存条目解析后是空商品。";
    case "cache_unavailable":
      return "当前 SDS 选择还没有可用的 baseline 缓存。";
    default:
      return "";
  }
}

export function getSDSBaselineReasonShortLabel(reasonCode?: string) {
  switch ((reasonCode ?? "").trim()) {
    case "missing_options":
      return "参数缺失";
    case "missing_parent_product":
      return "缺少父商品";
    case "missing_prototype_group":
      return "缺少 prototype group";
    case "missing_variant":
      return "缺少 variant";
    case "missing_design_type":
      return "缺少 design type";
    case "missing_printable_size":
      return "印刷尺寸缺失";
    case "missing_layer":
    case "layer_missing":
      return "图层缺失";
    case "login_unavailable":
      return "登录态不可用";
    case "login_in_progress":
      return "登录进行中";
    case "login_missing_credentials":
      return "登录凭证缺失";
    case "login_status_check_failed":
      return "登录检查失败";
    case "product_detail_check_failed":
      return "商品详情检查失败";
    case "product_detail_unavailable":
      return "商品详情不可用";
    case "design_surface_check_failed":
      return "设计面检查失败";
    case "design_surface_unavailable":
      return "设计面不可用";
    case "variant_mismatch":
      return "Variant 不匹配";
    case "prototype_group_mismatch":
      return "Prototype group 不匹配";
    case "prototype_group_check_failed":
      return "Prototype group 检查失败";
    case "prototype_group_unavailable":
      return "Prototype group 不可用";
    case "cache_repository_unavailable":
      return "缓存仓库不可用";
    case "cache_payload_missing":
      return "缓存缺少 payload";
    case "cache_payload_invalid":
      return "缓存 payload 损坏";
    case "cache_payload_empty":
      return "缓存商品为空";
    case "cache_unavailable":
      return "缓存缺失";
    default:
      return "";
  }
}

export function getSDSBaselineStatusLabel(status: SDSBaselineDisplayStatus) {
  switch (status) {
    case "ready":
      return "Baseline 已就绪";
    case "blocked":
      return "Baseline 已阻断";
    case "baseline_cached":
      return "Baseline 待校验";
    case "failed":
      return "Baseline 异常";
    case "missing":
      return "Baseline 缺失";
    default:
      return "Baseline 检查中";
  }
}

export function getSDSBaselineStatusBadgeVariant(
  status: SDSBaselineDisplayStatus,
): NonNullable<BadgeProps["variant"]> {
  switch (status) {
    case "ready":
      return "success";
    case "blocked":
    case "failed":
      return "danger";
    case "baseline_cached":
    case "missing":
      return "warning";
    default:
      return "neutral";
  }
}

export function buildGroupedSDSBaselineHelperText(input: {
  status: SDSBaselineDisplayStatus;
  reasonCode?: string;
  reason?: string;
}) {
  const reason = input.reason?.trim();
  if (reason) {
    return reason;
  }
  const reasonFromCode = getSDSBaselineReasonMessage(input.reasonCode);
  if (reasonFromCode) {
    return reasonFromCode;
  }
  switch (input.status) {
    case "loading":
      return "正在读取 baseline 状态，稍后就能判断是否可加入分组。";
    case "failed":
      return "baseline 检查失败，建议先排查这款 SDS 商品的缓存或转换链路。";
    case "blocked":
      return "这款商品的 baseline 校验被阻断，建议先修复模板、图层或 SDS 登录态问题。";
    case "baseline_cached":
      return "这款商品只完成了 baseline 缓存，暂时还不能加入 grouped 批量上品。";
    case "missing":
      return "这款商品还没有 baseline 缓存，暂时不能加入 grouped 批量上品。";
    default:
      return "";
  }
}

export function buildGroupedSDSBaselineActionLabel(input: {
  status: SDSBaselineDisplayStatus;
  reasonCode?: string;
  reason?: string;
}) {
  switch (input.status) {
    case "missing":
      return "回选并预热";
    case "failed":
      return "回选并重试";
    case "blocked":
      return "回选并修复";
    case "baseline_cached":
      return "回选并继续校验";
    case "loading":
      return "回选并等待";
    default:
      return "回选这个变体";
  }
}

export function buildGroupedSDSBaselineHandoff(input: {
  status: SDSBaselineDisplayStatus;
  reasonCode?: string;
  reason?: string;
}): Pick<SDSGroupedCandidateHandoff, "action" | "actionLabel" | "message"> | null {
  const reason = input.reason?.trim();
  const reasonFromCode = getSDSBaselineReasonMessage(input.reasonCode);
  const resolvedReason = reason || reasonFromCode;
  const normalizedReason = resolvedReason.toLowerCase();
  const isLoginBlocked =
    input.reasonCode === "login_missing_credentials" ||
    input.reasonCode === "login_unavailable" ||
    input.reasonCode === "login_in_progress" ||
    input.reasonCode === "login_status_check_failed" ||
    normalizedReason.includes("sds login") ||
    normalizedReason.includes("access token");
  switch (input.status) {
    case "missing":
      return {
        action: "warm_baseline",
        actionLabel: "一键预热并校验 baseline",
        message:
          resolvedReason ||
          "这款候选商品还没有 baseline 缓存。先在当前工作台完成一次生成或预热，再回来继续校验并加入 grouped 批量上品。",
      };
    case "baseline_cached":
      return {
        action: "warm_baseline",
        actionLabel: "继续 baseline 校验",
        message:
          resolvedReason ||
          "这款候选商品已经完成 baseline 缓存，但还没通过进一步校验。先继续校验，再回来加入 grouped 批量上品。",
      };
    case "blocked":
      if (isLoginBlocked) {
        return {
          action: "open_sds_login",
          actionLabel: "去处理 SDS 登录",
          message:
            resolvedReason ||
            "当前候选商品的 baseline 校验被 SDS 登录态阻断。请先处理 SDS 登录，再回来继续校验并加入 grouped 批量上品。",
        };
      }
      return {
        action: "focus_generate",
        actionLabel: "去生成区修复",
        message:
          resolvedReason ||
          "这款候选商品的 baseline 校验已被阻断。请先修复模板、图层或 SDS 登录态问题，再尝试加入 grouped 批量上品。",
      };
    case "failed":
      return {
        action: "warm_baseline",
        actionLabel: "重试 baseline 校验",
        message:
          resolvedReason ||
          "这款候选商品的 baseline 检查失败。请先重新发起校验，或排查 SDS 转标准商品链路后再尝试 grouped 批量上品。",
      };
    case "loading":
      return {
        action: "focus_generate",
        actionLabel: "去生成区查看",
        message:
          resolvedReason ||
          "这款候选商品的 baseline 状态还在检查中。稍等片刻，确认校验结果后再加入 grouped 批量上品。",
      };
    default:
      return null;
  }
}

export function buildRecentBatchBaselineAlert(input: {
  status?: string;
  reasonCode?: string;
  reason?: string;
}): SheinStudioRecentBatchAlert | null {
  if (!input.status || input.status === "ready") {
    return null;
  }
  const reason = input.reason?.trim();
  const reasonFromCode = getSDSBaselineReasonMessage(input.reasonCode);
  return {
    tone: "danger",
    reasonCode: input.reasonCode?.trim() || undefined,
    label:
      input.status === "baseline_cached"
        ? "Baseline 待校验"
        : "Baseline 校验未通过",
    detail:
      reason ||
      reasonFromCode ||
      (input.status === "baseline_cached"
        ? "组内仍有商品只完成了 baseline 缓存，尚未通过进一步校验。"
        : "组内仍有商品需要先完成 baseline 修复后再继续。"),
  };
}
