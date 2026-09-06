import { configuredSheinRecordsOrigin } from "./shein-records-origin";

/** Server-only capability hint. This never exposes the configured origin. */
export function isSheinRecordsAvailable(): boolean {
  return configuredSheinRecordsOrigin() !== null;
}
