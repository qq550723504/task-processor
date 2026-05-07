import type { SheinSubmissionReport } from "@/lib/types/listingkit";

export function sheinSubmissionActionLabel(action?: string | null) {
  switch (action) {
    case "image_upload":
      return "上传图片";
    case "save_draft":
      return "保存草稿";
    case "publish":
      return "正式发布";
    default:
      return action ?? "提交事件";
  }
}

export function sheinWorkflowStatusLabel(status?: string | null) {
  switch (status) {
    case "pending_confirmation":
      return "待确认";
    case "ready_to_submit":
      return "可提交";
    case "publishing":
      return "发布中";
    case "publish_failed":
      return "发布失败";
    case "published":
      return "已发布";
    case "draft_saved":
      return "草稿已保存";
    default:
      return status?.replaceAll("_", " ") ?? "未知状态";
  }
}

export function sheinSubmissionStatusLabel(status?: string | null) {
  switch (status) {
    case "success":
      return "成功";
    case "failed":
      return "失败";
    case "processing":
      return "处理中";
    default:
      return status ?? "未知";
  }
}

export function sheinSubmissionRemoteStatusLabel(status?: string | null) {
  switch (status) {
    case "confirmed":
      return "远端已确认";
    case "pending":
      return "待 SHEIN 确认";
    case "failed":
      return "确认失败";
    default:
      return status?.replaceAll("_", " ") ?? "远端未确认";
  }
}

export function sheinSubmitPhaseLabel(phase?: string | null) {
  switch (phase) {
    case "validate":
      return "提交前检查";
    case "prepare_product":
      return "准备商品资料";
    case "upload_images":
      return "上传图片";
    case "pre_validate":
      return "SHEIN 预校验";
    case "submit_remote":
      return "提交 SHEIN";
    case "persist_result":
      return "保存提交结果";
    case "confirm_remote":
      return "确认远端记录";
    default:
      return phase ?? null;
  }
}

function latestSubmissionRecord(submission?: SheinSubmissionReport | null) {
  switch (submission?.last_action) {
    case "save_draft":
      return submission.save_draft;
    case "publish":
      return submission.publish;
    default:
      return null;
  }
}

export function sheinLatestSubmissionTitle(
  submission?: SheinSubmissionReport | null,
) {
  const action = submission?.last_action;
  const status = submission?.last_status;
  const success =
    status === "success" ||
    (status === "unknown" && submission?.last_result?.success === true);
  const failed =
    status === "failed" ||
    submission?.last_result?.success === false ||
    Boolean(submission?.last_error);

  if (action === "save_draft") {
    if (success) return "已保存到 SHEIN 草稿箱";
    if (failed) return "保存草稿失败";
    return "保存草稿处理中";
  }
  if (action === "publish") {
    if (success) return "已提交到 SHEIN";
    if (failed) return "发布失败";
    return "发布处理中";
  }
  if (action === "image_upload") {
    if (success) return "图片已上传到 SHEIN";
    if (failed) return "图片上传失败";
    return "图片上传处理中";
  }
  if (failed) return "提交失败";
  if (success) return "提交成功";
  return "暂无明确提交结果";
}

export function sheinLatestSubmissionSummary(
  submission?: SheinSubmissionReport | null,
) {
  const title = sheinLatestSubmissionTitle(submission);
  if (submission?.last_action === "save_draft" && title.includes("已保存")) {
    return "商品资料已进入 SHEIN 草稿箱，不会直接上架。";
  }
  if (submission?.last_action === "publish" && title.includes("已提交")) {
    if (submission.remote_status === "confirmed") {
      return "商品资料已提交到 SHEIN，远端记录已确认。";
    }
    if (submission.remote_status === "pending") {
      return "商品资料已提交到 SHEIN，远端记录暂未查询到，请稍后以 SHEIN 后台状态为准。";
    }
    if (submission.remote_status === "failed") {
      return "商品资料已提交到 SHEIN，但远端状态确认失败，请以 SHEIN 后台最终状态为准。";
    }
    return "商品资料已提交到 SHEIN 发布接口，请以 SHEIN 后台最终状态为准。";
  }
  if (title.includes("失败")) {
    const record = latestSubmissionRecord(submission);
    const details = [
      record?.phase ? `失败阶段：${sheinSubmitPhaseLabel(record.phase)}` : null,
      record?.request_id ? `Request ID: ${record.request_id}` : null,
    ].filter(Boolean);
    const suffix = "请根据待处理问题修复后再次提交。";
    return details.length ? `${details.join("。")}。${suffix}` : suffix;
  }
  return submission?.last_result?.message ?? "";
}
