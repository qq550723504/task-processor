export type RequestLogFields = Record<string, string | number | boolean | undefined>;

export function newRequestLogId() {
  return Math.random().toString(36).slice(2, 10);
}

export function logRequestInfo(message: string, fields: RequestLogFields) {
  console.info(formatRequestLog("info", message, fields));
}

export function logRequestWarn(message: string, fields: RequestLogFields) {
  console.warn(formatRequestLog("warn", message, fields));
}

function formatRequestLog(
  level: "info" | "warn",
  message: string,
  fields: RequestLogFields,
) {
  const details = Object.entries(fields)
    .filter(([, value]) => value !== undefined)
    .map(([key, value]) => `${key}=${JSON.stringify(value)}`)
    .join(" ");
  return `[listingkit-ui] level=${level} message=${JSON.stringify(message)} ${details}`;
}
