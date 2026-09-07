import path from "node:path";

export function publicBrowserOrigins(manifest) {
  return { web: manifest.origins.web, go: manifest.origins.go, issuer: manifest.origins.issuer };
}

export function assertBrowserDiagnosticsDisabled(environment) {
  if (Object.entries(environment).some(([key, value]) => value && (/^(DEBUG|PWDEBUG|DEBUG_FILE)$/i.test(key) || /^(PW|PLAYWRIGHT).*(DEBUG|LOG|TRACE)/i.test(key)))) {
    throw new Error("issue358_browser_diagnostics_forbidden");
  }
}

export function classifyLateResponseDelivery(delivered, cancellationError) {
  if (delivered) return "delivered";
  if (cancellationError === "net::ERR_ABORTED") return "cancelled";
  throw new Error("issue358_late_response_unproven");
}

export function classifyRevocationRead({ status, confirmedAt, requestStartedAt }) {
  if (status === 403) return "denied";
  if (status === 200 && requestStartedAt - confirmedAt <= 60000) return "cached";
  throw new Error("issue358_revocation_not_converged");
}

export async function withOwnerControlRestored(mutate, operation, restore) {
  try {
    await mutate();
    return await operation();
  } finally { await restore(); }
}

// Consumer of #357 schema v1, not a second runtime/seed or application DTO.
export function validateBrowserHandoff(value, { manifestPath, runtimeSha, webSha, temporaryRoot }) {
  try {
    const require = condition => { if (!condition) throw new Error(); };
    const identifier = value => typeof value === "string" && /^[A-Za-z0-9_-]{1,128}$/.test(value);
    require(value && value.schemaVersion === "issue357-v1" && value.status === "ready");
    require(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value.runId));
    require(/^[0-9a-f]{40}$/.test(runtimeSha) && /^[0-9a-f]{40}$/.test(webSha));
    require(value.sourceSha === runtimeSha && value.webSha === webSha);
    require(value.sourceDirty === false && value.webDirty === false);
    require(!["sessions", "cookies", "storageState", "sessionToken"].some(key => key in value));
    const root = path.resolve(temporaryRoot, "task-processor-issue357", value.runId);
    require(path.resolve(manifestPath) === path.join(root, "manifest.json"));
    const origins = ["web", "go", "issuer"].map(key => {
      const raw = value.origins[key];
      const url = new URL(raw);
      require(url.protocol === "http:" && ["localhost", "127.0.0.1"].includes(url.hostname));
      require(url.port && !url.username && !url.password && raw === url.origin);
      return url;
    });
    require(new Set(origins.map(url => url.port)).size === 3);
    require(identifier(value.instanceId) && identifier(value.projectId));
    const organizations = ["A", "B", "C", "Empty", "D"].map(key => value.organizations[key]);
    require(organizations.every(org => identifier(org.id) && typeof org.name === "string" && org.name.length > 0));
    require(new Set(organizations.map(org => org.id)).size === 5);
    const users = ["admin", "viewer", "no-org"].map(key => value.users[key]);
    require(users.every(user => identifier(user.id) && user.homeOrganizationId === value.organizations.A.id));
    require(new Set(users.map(user => user.id)).size === 3);
    for (const user of users) {
      require(path.isAbsolute(user.credentialFile));
      const relative = path.relative(root, path.resolve(user.credentialFile));
      require(relative && !relative.startsWith("..") && !path.isAbsolute(relative) && relative.endsWith(".json"));
    }
    return value;
  } catch {
    // Never echo manifest content, origins containing credentials or parser errors.
    throw new Error("issue358_handoff_invalid");
  }
}
