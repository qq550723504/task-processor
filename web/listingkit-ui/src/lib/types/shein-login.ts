export type SheinLoginFailureSummary = {
  error_code?: string;
  error_message?: string;
  page_state?: string;
  action_key?: string;
  action_message?: string;
  artifact_path?: string;
  captured_at?: string;
  stage?: string;
  url?: string;
  title?: string;
  login_error?: string;
  waiting_for_verify_code?: boolean;
};

export type SheinLoginFailureDetail = SheinLoginFailureSummary & {
  on_login_page?: boolean;
  request_failure_modal?: boolean;
  login_form_visible?: boolean;
  seller_hub_visible?: boolean;
  verification_visible?: boolean;
  permission_visible?: boolean;
  agreement_visible?: boolean;
  credential_error_visible?: boolean;
  body_text?: string;
  selector_states?: Record<string, boolean>;
  network_payloads?: Array<Record<string, unknown>>;
};

export type SheinLoginRecommendedAction = {
  key?: string;
  message?: string;
};

export type SheinLoginAccount = {
  store_id: number;
  tenant_id: number;
  username?: string;
  login_url?: string;
  proxy?: string;
  shop_name?: string;
  platform?: string;
  store_name?: string;
};

export type SheinLoginAccountStatus = {
  account: SheinLoginAccount;
  has_cookie: boolean;
  cookie_ttl?: number;
  waiting_for_verify_code: boolean;
  last_login_time?: string;
  login_in_progress: boolean;
  last_failure?: SheinLoginFailureSummary;
  recommended_action?: SheinLoginRecommendedAction;
};

export type SheinLoginWarehouse = {
  warehouse_code?: string;
  warehouse_name?: string;
  sale_country_list?: string[];
  warehouse_type?: number;
};

export type SheinLoginAccountsResponse = {
  success: boolean;
  data?: SheinLoginAccountStatus[];
  message?: string;
};
