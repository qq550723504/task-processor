terraform {
  required_version = "= 1.12.6"
  required_providers {
    zitadel = {
      source  = "zitadel/zitadel"
      version = "= 3.4.0"
    }
  }
}

variable "bootstrap_pat" {
  type      = string
  sensitive = true
}
variable "operator_password" {
  type      = string
  sensitive = true
}

provider "zitadel" {
  domain       = "localhost"
  port         = "18443"
  access_token = var.bootstrap_pat
  insecure     = false
}

resource "zitadel_org" "signup" {
  name = "Local Referrals Signup"
}

resource "zitadel_machine_user" "referral_provider" {
  org_id      = zitadel_org.signup.id
  user_name   = "local-referral-provider"
  name        = "Local referral provider"
  description = "Local Compose only; ORGANIZATION USER MANAGER for referral registration"
  with_secret = false
}

resource "zitadel_org_member" "referral_provider" {
  org_id  = zitadel_org.signup.id
  user_id = zitadel_machine_user.referral_provider.id
  roles   = ["ORG_USER_MANAGER"]
}

resource "zitadel_personal_access_token" "referral_provider" {
  org_id          = zitadel_org.signup.id
  user_id         = zitadel_machine_user.referral_provider.id
  expiration_date = "2099-01-01T00:00:00Z"
  depends_on      = [zitadel_org_member.referral_provider]
}

resource "zitadel_human_user" "bootstrap_operator" {
  org_id                       = zitadel_org.signup.id
  user_name                    = "local-bootstrap-operator@localhost"
  first_name                   = "Local"
  last_name                    = "Bootstrap Operator"
  display_name                 = "Local Bootstrap Operator"
  email                        = "local-bootstrap-operator@localhost"
  is_email_verified            = true
  initial_password             = var.operator_password
  initial_skip_password_change = true
}

resource "zitadel_project" "listingkit" {
  org_id                   = zitadel_org.signup.id
  name                     = "ListingKit Local Referrals"
  project_role_assertion   = true
  project_role_check       = false
  has_project_check        = false
  private_labeling_setting = "PRIVATE_LABELING_SETTING_ENFORCE_PROJECT_RESOURCE_OWNER_POLICY"
}

resource "zitadel_project_role" "operator" {
  org_id       = zitadel_org.signup.id
  project_id   = zitadel_project.listingkit.id
  role_key     = "listingkit_operator"
  display_name = "ListingKit Operator"
  group        = "ListingKit"
}

resource "zitadel_project_role" "admin" {
  org_id       = zitadel_org.signup.id
  project_id   = zitadel_project.listingkit.id
  role_key     = "listingkit_admin"
  display_name = "ListingKit Admin"
  group        = "ListingKit"
}

resource "zitadel_project_role" "platform_admin" {
  org_id       = zitadel_org.signup.id
  project_id   = zitadel_project.listingkit.id
  role_key     = "platform_admin"
  display_name = "Platform Admin"
  group        = "ListingKit"
}

resource "zitadel_user_grant" "bootstrap_operator" {
  org_id     = zitadel_org.signup.id
  project_id = zitadel_project.listingkit.id
  user_id    = zitadel_human_user.bootstrap_operator.id
  role_keys  = ["listingkit_operator", "listingkit_admin", "platform_admin"]
  depends_on = [
    zitadel_project_role.operator,
    zitadel_project_role.admin,
    zitadel_project_role.platform_admin,
  ]
}

resource "zitadel_application_api" "current_application" {
  org_id           = zitadel_org.signup.id
  project_id       = zitadel_project.listingkit.id
  name             = "Current Application Local"
  auth_method_type = "API_AUTH_METHOD_TYPE_BASIC"
}

resource "zitadel_application_oidc" "listingkit_ui" {
  org_id                       = zitadel_org.signup.id
  project_id                   = zitadel_project.listingkit.id
  name                         = "ListingKit UI Local"
  redirect_uris                = ["https://localhost:18444/api/auth/callback/zitadel"]
  post_logout_redirect_uris    = ["https://localhost:18444"]
  response_types               = ["OIDC_RESPONSE_TYPE_CODE"]
  grant_types                  = ["OIDC_GRANT_TYPE_AUTHORIZATION_CODE", "OIDC_GRANT_TYPE_REFRESH_TOKEN"]
  app_type                     = "OIDC_APP_TYPE_WEB"
  auth_method_type             = "OIDC_AUTH_METHOD_TYPE_BASIC"
  version                      = "OIDC_VERSION_1_0"
  clock_skew                   = "0s"
  dev_mode                     = false
  access_token_type            = "OIDC_TOKEN_TYPE_BEARER"
  access_token_role_assertion  = false
  id_token_role_assertion      = true
  id_token_userinfo_assertion  = false
  additional_origins           = []
  skip_native_app_success_page = false
}

output "provider_pat" {
  value     = zitadel_personal_access_token.referral_provider.token
  sensitive = true
}
output "signup_org_id" { value = zitadel_org.signup.id }
output "project_id" { value = zitadel_project.listingkit.id }
output "api_client_id" {
  value     = zitadel_application_api.current_application.client_id
  sensitive = true
}
output "api_client_secret" {
  value     = zitadel_application_api.current_application.client_secret
  sensitive = true
}
output "oidc_client_id" {
  value     = zitadel_application_oidc.listingkit_ui.client_id
  sensitive = true
}
output "oidc_client_secret" {
  value     = zitadel_application_oidc.listingkit_ui.client_secret
  sensitive = true
}
