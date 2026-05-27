import type {
  SheinPreviewPayload,
  SheinReadinessItem,
  SheinSubmissionReport,
  SheinSubmitReadiness,
} from "@/lib/types/listingkit";
import { getSheinSubmissionState } from "@/lib/listingkit/semantic-fields";

export type CustomerIssueCategory =
  | "图片问题"
  | "类目问题"
  | "普通属性问题"
  | "销售属性问题"
  | "价格/库存问题"
  | "提交接口问题"
  | "其他问题";

export type CustomerIssueSeverity = "blocking" | "warning" | "error";

export type CustomerIssueActionKey =
  | "store_login"
  | "images"
  | "category"
  | "attributes"
  | "sale_attributes"
  | "pricing";

export type CustomerIssue = {
  category: CustomerIssueCategory;
  title: string;
  message: string;
  severity: CustomerIssueSeverity;
  actionLabel: string;
  actionKey?: CustomerIssueActionKey;
  rawText: string;
};

type IssueTemplate = Omit<CustomerIssue, "severity" | "rawText">;

function textOfReadinessItem(item: SheinReadinessItem) {
  return [
    item.label,
    item.message,
    item.reason?.summary,
    item.suggested_action,
    item.field_paths?.join(" "),
  ]
    .filter(Boolean)
    .join(" · ")
    .trim();
}

function makeIssue(
  template: IssueTemplate,
  severity: CustomerIssueSeverity,
  rawText: string,
): CustomerIssue {
  return {
    ...template,
    severity,
    rawText: rawText || template.message,
  };
}

function byKey(key?: string | null): IssueTemplate | null {
  const normalized = (key ?? "").toLowerCase();
  if (!normalized) {
    return null;
  }
  if (normalized === "final_review") {
    return {
      category: "其他问题",
      title: "等待最终确认",
      message: "当前页面就是最终确认页。核对无误后，可以直接保存草稿或发布到 SHEIN。",
      actionLabel: "继续最终确认",
    };
  }
  if (normalized.includes("cookie") || normalized.includes("login")) {
    return {
      category: "提交接口问题",
      title: "SHEIN 店铺需要重新登录",
      message:
        "当前店铺 cookie 不可用，系统无法在线获取 SHEIN 类目、属性和销售属性模板。请重新登录店铺或刷新 cookie 后重新生成/重试。",
      actionLabel: "去登录店铺",
      actionKey: "store_login",
    };
  }
  if (normalized.includes("image") || normalized.includes("preview_product")) {
    return {
      category: "图片问题",
      title: "图片资料还不完整",
      message: "请检查最终图片、主图、色块图和 SKC 图片是否已设置。",
      actionLabel: "去检查图片",
      actionKey: "images",
    };
  }
  if (normalized.includes("sale_attribute") || normalized.includes("variant")) {
    return {
      category: "销售属性问题",
      title: "销售属性需要确认",
      message: "请确认 SHEIN 主规格、其他规格和每个变体的属性值。",
      actionLabel: "去确认销售属性",
      actionKey: "sale_attributes",
    };
  }
  if (normalized.includes("attribute")) {
    return {
      category: "普通属性问题",
      title: "商品属性需要补齐",
      message: "请补齐 SHEIN 模板要求的商品属性，例如材质、型号或电池信息。",
      actionLabel: "去确认属性",
      actionKey: "attributes",
    };
  }
  if (normalized.includes("category")) {
    return {
      category: "类目问题",
      title: "类目需要确认",
      message: "当前 SHEIN 类目或类目模板还需要人工确认。",
      actionLabel: "去确认类目",
      actionKey: "category",
    };
  }
  if (
    normalized.includes("price") ||
    normalized.includes("stock") ||
    normalized.includes("inventory")
  ) {
    return {
      category: "价格/库存问题",
      title: "价格或库存需要检查",
      message: "请在最终确认页检查 SKU 售价、库存和包装相关信息。",
      actionLabel: "去检查价格和 SKU",
      actionKey: "pricing",
    };
  }
  return null;
}

function byText(rawText: string): IssueTemplate {
  const text = rawText.toLowerCase();

  if (
    rawText.includes("店铺 cookie") ||
    rawText.includes("cookie 不可用") ||
    text.includes("cookies are unavailable")
  ) {
    return {
      category: "提交接口问题",
      title: "SHEIN 店铺需要重新登录",
      message:
        "当前店铺 cookie 不可用，系统无法在线获取 SHEIN 类目、属性和销售属性模板。请重新登录店铺或刷新 cookie 后重新生成/重试。",
      actionLabel: "去登录店铺",
      actionKey: "store_login",
    };
  }
  if (text.includes("spu_name")) {
    return {
      category: "提交接口问题",
      title: "SHEIN SPU 编码填写不正确",
      message:
        "新增商品时 spu_name 应保持为空，由 SHEIN 系统生成；请检查提交数据后重新保存或发布。",
      actionLabel: "查看提交记录",
    };
  }
  if (rawText.includes("方形图必须有一个")) {
    return {
      category: "图片问题",
      title: "缺少方形图",
      message: "SHEIN 要求至少有一张方形图。请在图片区设置主图或方形图角色。",
      actionLabel: "去设置图片",
      actionKey: "images",
    };
  }
  if (rawText.includes("图片只能上传一张")) {
    return {
      category: "图片问题",
      title: "图片角色数量不符合要求",
      message: "某个图片位置只允许上传一张。请检查主图、色块图和图库角色是否重复。",
      actionLabel: "去检查图片",
      actionKey: "images",
    };
  }
  if (rawText.includes("色块图") || rawText.includes("缺少色块")) {
    return {
      category: "图片问题",
      title: "缺少色块图",
      message: "多颜色或多 SKC 商品需要色块图。请在图片区为对应颜色标记色块图。",
      actionLabel: "去标记色块图",
      actionKey: "images",
    };
  }
  if (
    rawText.includes("主规格") ||
    rawText.includes("销售属性") ||
    text.includes("shelf_way")
  ) {
    return {
      category: "销售属性问题",
      title: "销售规格不符合 SHEIN 要求",
      message: "请确认主规格和其他规格的选择，确保每个 SKC/SKU 都映射到 SHEIN 允许的属性值。",
      actionLabel: "去确认销售属性",
      actionKey: "sale_attributes",
    };
  }
  if (
    rawText.includes("模板属性为必填项") ||
    rawText.includes("材质") ||
    rawText.includes("产品型号") ||
    rawText.includes("包含电池") ||
    text.includes("required attribute")
  ) {
    return {
      category: "普通属性问题",
      title: "商品属性需要补齐",
      message: "请补齐 SHEIN 模板要求的商品属性，例如材质、型号或电池信息。",
      actionLabel: "去确认属性",
      actionKey: "attributes",
    };
  }
  if (rawText.includes("类目") || text.includes("category")) {
    return {
      category: "类目问题",
      title: "类目匹配需要确认",
      message: "当前类目可能不符合 SHEIN 模板要求，请确认推荐类目后再提交。",
      actionLabel: "去确认类目",
      actionKey: "category",
    };
  }
  if (
    rawText.includes("价格") ||
    rawText.includes("库存") ||
    text.includes("price") ||
    text.includes("stock") ||
    text.includes("quantity_info")
  ) {
    return {
      category: "价格/库存问题",
      title: "价格或库存信息不完整",
      message: "请检查最终确认页中的 SKU 售价、库存、包装和数量信息。",
      actionLabel: "去检查价格和 SKU",
      actionKey: "pricing",
    };
  }
  if (rawText.includes("图片") || text.includes("image")) {
    return {
      category: "图片问题",
      title: "图片资料需要检查",
      message: "请检查最终提交图片、图片角色和 SHEIN 图片上传结果。",
      actionLabel: "去检查图片",
      actionKey: "images",
    };
  }

  return {
    category: "其他问题",
    title: "存在未识别的问题",
    message: "请查看原始 SHEIN 返回或提交日志，确认需要人工处理的内容。",
    actionLabel: "查看原始返回",
  };
}

function issueFromReadinessItem(
  item: SheinReadinessItem,
  severity: CustomerIssueSeverity,
) {
  const rawText = textOfReadinessItem(item) || item.key || "未知检查项";
  const keyTemplate = byKey(item.key);
  const textTemplate = byText(rawText);
  return makeIssue(
    keyTemplate ?? textTemplate,
    item.key === "final_review" ? "warning" : severity,
    rawText,
  );
}

function appendReadinessIssues(
  issues: CustomerIssue[],
  readiness?: SheinSubmitReadiness | null,
) {
  for (const item of readiness?.blocking_items ?? []) {
    issues.push(issueFromReadinessItem(item, "blocking"));
  }
  for (const item of readiness?.warning_items ?? []) {
    issues.push(issueFromReadinessItem(item, "warning"));
  }
}

function appendSubmissionIssues(
  issues: CustomerIssue[],
  submission?: SheinSubmissionReport | null,
) {
  const notes = submission?.last_result?.validation_notes ?? [];
  for (const note of notes) {
    if (note?.trim()) {
      issues.push(makeIssue(byText(note), "error", note));
    }
  }
  const lastError = submission?.last_error?.trim();
  if (lastError) {
    issues.push(makeIssue(byText(lastError), "error", lastError));
  }
}

export function buildSheinCustomerIssues(
  shein?: Pick<SheinPreviewPayload, "submit_readiness" | "submission" | "submission_state"> | null,
) {
  const issues: CustomerIssue[] = [];
  appendReadinessIssues(issues, shein?.submit_readiness);
  appendSubmissionIssues(issues, getSheinSubmissionState(shein));

  const seen = new Set<string>();
  return issues.filter((issue) => {
    const key = [
      issue.category,
      issue.title,
      issue.actionKey ?? "",
      issue.message,
    ].join("|");
    if (seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}
