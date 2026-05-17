export type PromptTemplate = {
  id?: number;
  tenant_id?: string;
  key: string;
  content: string;
  version?: string;
  enabled?: boolean;
  created_at?: string;
  updated_at?: string;
};

export type PromptTemplateListResponse = {
  items: PromptTemplate[];
};

export type PromptTemplateScopeDefinition = {
  id: string;
  label: string;
  description?: string;
};

export type PromptTemplateVariableDefinition = {
  key: string;
  label: string;
  description?: string;
};

export type PromptTemplateSchema = {
  key: string;
  label: string;
  description?: string;
  group: string;
  group_label: string;
  category: string;
  category_label: string;
  supported_scopes?: PromptTemplateScopeDefinition[];
  variables?: PromptTemplateVariableDefinition[];
  has_default_content: boolean;
  supports_tenant_override: boolean;
};

export type PromptTemplateCatalogResponse = {
  items: PromptTemplateSchema[];
};
