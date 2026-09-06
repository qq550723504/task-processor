export function configuredSheinRecordsOrigin(): string | null {
  const raw = process.env.SHEIN_RECORDS_API_ORIGIN;
  if (!raw) return null;
  try {
    const url = new URL(raw);
    return ["http:", "https:"].includes(url.protocol) && !url.username && !url.password && (raw === url.origin || raw === `${url.origin}/`) ? url.origin : null;
  } catch { return null; }
}
