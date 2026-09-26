terraform {
  required_version = "= 1.12.6"
  required_providers {
    zitadel = {
      source  = "zitadel/zitadel"
      version = "= 3.4.0"
    }
  }
}

variable "provider_pat" {
  type      = string
  sensitive = true
}
variable "signup_org_id" { type = string }
variable "probe_password" {
  type      = string
  sensitive = true
}

provider "zitadel" {
  domain       = "localhost"
  port         = "18443"
  access_token = var.provider_pat
  insecure     = false
}

# Applying and destroying this resource requires the configured provider
# machine to create, read, and delete a user through the v2 user API.
resource "zitadel_human_user" "permission_probe" {
  org_id                       = var.signup_org_id
  user_name                    = "local-referral-permission-probe@localhost"
  first_name                   = "Local"
  last_name                    = "Permission Probe"
  email                        = "local-referral-permission-probe@localhost"
  is_email_verified            = false
  initial_password             = var.probe_password
  initial_skip_password_change = true
}
