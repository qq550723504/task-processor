export type LoginEntry = "generic_login" | "otp_login" | "password_login";

const LOGIN_ENTRY_BY_METHOD = {
  otp: "otp_login",
  password: "password_login",
} as const satisfies Record<string, LoginEntry>;

type SpecializedLoginEntry = (typeof LOGIN_ENTRY_BY_METHOD)[keyof typeof LOGIN_ENTRY_BY_METHOD];

const LOGIN_METHOD_BY_ENTRY = Object.fromEntries(
  Object.entries(LOGIN_ENTRY_BY_METHOD).map(([method, entry]) => [entry, method]),
) as Record<SpecializedLoginEntry, keyof typeof LOGIN_ENTRY_BY_METHOD>;

/**
 * Selects only public login entries defined by the frozen account-entry
 * contract. Missing, repeated, and unknown methods always retain the generic
 * ZITADEL Login V2 chooser.
 */
export function resolveLoginEntry(methods: readonly string[]): LoginEntry {
  if (methods.length !== 1) {
    return "generic_login";
  }
  const method = methods[0];
  return Object.hasOwn(LOGIN_ENTRY_BY_METHOD, method)
    ? LOGIN_ENTRY_BY_METHOD[method as keyof typeof LOGIN_ENTRY_BY_METHOD]
    : "generic_login";
}

export function loginMethodForEntry(entry: LoginEntry) {
  return entry === "generic_login" ? undefined : LOGIN_METHOD_BY_ENTRY[entry];
}

/**
 * Phone-specific entries remain closed until the pinned ZITADEL/Login V2
 * staging flow and the thin-fork entry handoff have both been verified.
 * Keeping this gate in code prevents configuration alone from advertising a
 * flow that cannot yet complete OIDC and Auth.js session creation.
 */
export function isLoginEntryAvailable(entry: LoginEntry) {
  return entry === "generic_login";
}

export function normalizeReturnTo(value: string | null) {
  if (
    !value ||
    !value.startsWith("/") ||
    value.startsWith("//") ||
    value.includes("\\")
  ) {
    return "/";
  }
  try {
    decodeURIComponent(value);
  } catch {
    return "/";
  }
  if (!isAllowedReturnToPath(value)) {
    return "/";
  }
  return value;
}

export function resolveAuthRedirect(
  url: string,
  baseUrl: string,
  issuerUrl?: string,
) {
  let base: URL;
  try {
    base = new URL(baseUrl);
  } catch {
    return "/";
  }

  if (url.startsWith("/")) {
    return new URL(normalizeReturnTo(url), base).toString();
  }

  try {
    const target = new URL(url);
    if (target.origin === base.origin) {
      const localTarget = `${target.pathname}${target.search}${target.hash}`;
      return new URL(normalizeReturnTo(localTarget), base).toString();
    }
    if (issuerUrl && target.origin === new URL(issuerUrl).origin) {
      return target.toString();
    }
  } catch {
    // Invalid and non-URL values fall through to the local safe default.
  }

  return new URL("/", base).toString();
}

function isAllowedReturnToPath(value: string) {
  const localOrigin = "http://localhost";
  let pathname: string;
  try {
    const returnToUrl = new URL(value, localOrigin);
    if (returnToUrl.origin !== localOrigin) {
      return false;
    }
    pathname = returnToUrl.pathname;
  } catch {
    return false;
  }
  return (
    pathname === "/" ||
    pathname === "/listing-kits" ||
    pathname.startsWith("/listing-kits/") ||
    pathname === "/workbench" ||
    pathname.startsWith("/workbench/")
  );
}
