import {
  MemberResourceError,
  type MemberResourceEntry,
  type MemberTransferInput,
} from "@/lib/api/member-resources";
export const roles: Record<string, string> = {
  listingkit_admin: "企业管理员",
  listingkit_operator: "运营",
  listingkit_viewer: "只读成员",
};
export const name = (member: MemberResourceEntry) =>
  member.displayName || member.loginName || member.memberId;
export const live = (member: MemberResourceEntry) => member.state === "active";
export const position = (
  member: MemberResourceEntry,
  type: MemberTransferInput["resourceType"],
) => (type === "data_row" ? member.dataRows : member.periods);
export function failure(error: unknown) {
  const labels: Record<string, string> = {
    CONFLICT: "余额或版本已变化，请刷新后重试。",
    QUOTE_EXPIRED: "报价已失效，请重新换算。",
    RESOURCE_INSUFFICIENT_BALANCE: "可用余额不足。",
    RESOURCE_DEBT_OUTSTANDING: "企业有待偿还额度，当前不能分配。",
    FORBIDDEN: "当前成员状态或权限不允许此操作。",
    DATA_PRICE_UNAVAILABLE: "当前未配置可用的数据价格。",
  };
  return error instanceof MemberResourceError
    ? (labels[error.code] ?? "操作未完成，请确认当前企业与权限后重试。")
    : "输入或响应无效，请重新确认。";
}
