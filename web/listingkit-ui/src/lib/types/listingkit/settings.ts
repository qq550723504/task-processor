export type ListingKitSettingsHealthStatus =
  | "ready"
  | "warning"
  | "blocked"
  | "unknown";

export type ListingKitSettingsHealthItem = {
  key: string;
  label: string;
  status: ListingKitSettingsHealthStatus;
  message: string;
  impact?: string[];
  action?: string;
};

export type ListingKitSettingsHealthPage = {
  status: ListingKitSettingsHealthStatus;
  items: ListingKitSettingsHealthItem[];
};
