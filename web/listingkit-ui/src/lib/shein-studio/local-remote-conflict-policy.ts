export type LocalRemoteDraftState =
  | "local_only"
  | "remote_only"
  | "local_newer"
  | "remote_newer"
  | "synchronized"
  | "conflict";

export function classifyLocalRemoteDraftState({
  localUpdatedAt,
  remoteUpdatedAt,
}: {
  localUpdatedAt?: string;
  remoteUpdatedAt?: string;
}): LocalRemoteDraftState {
  const localTime = parseTimestamp(localUpdatedAt);
  const remoteTime = parseTimestamp(remoteUpdatedAt);
  const hasLocal = Boolean(localUpdatedAt?.trim());
  const hasRemote = Boolean(remoteUpdatedAt?.trim());

  if (hasLocal && !hasRemote && Number.isFinite(localTime)) {
    return "local_only";
  }
  if (!hasLocal && hasRemote && Number.isFinite(remoteTime)) {
    return "remote_only";
  }
  if (!Number.isFinite(localTime) || !Number.isFinite(remoteTime)) {
    return "conflict";
  }
  if (localTime > remoteTime) {
    return "local_newer";
  }
  if (remoteTime > localTime) {
    return "remote_newer";
  }
  return "synchronized";
}

export function shouldUseLocalDraftOverRemote(input: {
  localUpdatedAt?: string;
  remoteUpdatedAt?: string;
}) {
  const state = classifyLocalRemoteDraftState(input);
  return state === "local_only" || state === "local_newer";
}

export function pickLocalStringValue(
  localValue: string | undefined,
  remoteValue: string | undefined,
) {
  return localValue?.trim() ? localValue : (remoteValue ?? "");
}

export function pickLocalArrayValue<T>(
  localValue: T[] | undefined,
  remoteValue: T[] | undefined,
) {
  return ((localValue?.length ?? 0) > 0 ? localValue : (remoteValue ?? [])) as T[];
}

function parseTimestamp(value?: string) {
  if (!value?.trim()) {
    return Number.NaN;
  }
  return new Date(value).getTime();
}
