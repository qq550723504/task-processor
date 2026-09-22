export type AccountPageKind = "overview" | "profile" | "profile-settings" | "profile-business" | "profile-verification" | "organization";

export function accountPagePath(page: AccountPageKind) {
  return page === "overview" ? "/workbench/account" : page === "profile" ? "/workbench/account/profile" : page === "profile-settings" ? "/workbench/account/profile/settings" : page === "profile-business" ? "/workbench/account/profile/business" : page === "profile-verification" ? "/workbench/account/profile/verification" : "/workbench/account/organization";
}
