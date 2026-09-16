terraform {
  required_version = "= 1.12.6"
  required_providers {
    zitadel = { source = "zitadel/zitadel", version = "= 3.4.0" }
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
resource "zitadel_org" "trial" { name = "Local Acquisition Trial" }
resource "zitadel_human_user" "operator" {
  org_id                       = zitadel_org.trial.id
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
  org_id                   = zitadel_org.trial.id
  name                     = "ListingKit Local Acquisition"
  project_role_assertion   = true
  project_role_check       = false
  has_project_check        = false
  private_labeling_setting = "PRIVATE_LABELING_SETTING_ENFORCE_PROJECT_RESOURCE_OWNER_POLICY"
}
resource "zitadel_project_role" "operator" {
  org_id       = zitadel_org.trial.id
  project_id   = zitadel_project.listingkit.id
  role_key     = "listingkit_operator"
  display_name = "ListingKit Operator"
  group        = "ListingKit"
}
resource "zitadel_user_grant" "operator" {
  org_id     = zitadel_org.trial.id
  project_id = zitadel_project.listingkit.id
  user_id    = zitadel_human_user.operator.id
  role_keys  = ["listingkit_operator"]
  depends_on = [zitadel_project_role.operator]
}
resource "zitadel_application_api" "current_application" {
  org_id           = zitadel_org.trial.id
  project_id       = zitadel_project.listingkit.id
  name             = "Current Application Local"
  auth_method_type = "API_AUTH_METHOD_TYPE_BASIC"
}
resource "zitadel_application_oidc" "listingkit_ui" {
  org_id                       = zitadel_org.trial.id
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
