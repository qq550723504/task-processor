"use client";

export const listingKitSettingsKeys = {
  root: ["listingkit", "settings"] as const,
  namespace(namespace: string, ...parts: Array<string | number | undefined>) {
    return [
      ...this.root,
      namespace,
      ...parts.filter((part) => part !== undefined && part !== ""),
    ] as const;
  },
  zitadelSession() {
    return this.namespace("zitadel-session");
  },
  metadataIndex() {
    return this.namespace("metadata-index");
  },
  schema(namespace: string) {
    return this.namespace("schema", namespace);
  },
  aiClient(scope: string, clientName: string, userId = "") {
    return this.namespace("ai", scope, clientName, userId);
  },
  prompts() {
    return this.namespace("prompts");
  },
} as const;
