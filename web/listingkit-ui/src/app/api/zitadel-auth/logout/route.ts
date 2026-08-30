import { serverAuth, signOut } from "@/auth";
import {
  fetchZitadelDiscovery,
  getZitadelAuthOptions,
  resolvePublicAppOrigin,
} from "@/lib/server/zitadel-auth";
import { readZitadelServerIDToken } from "@/lib/server/zitadel-server-token";

export const dynamic = "force-dynamic";

const authenticatedGET = serverAuth(async (request) => {
  const options = getZitadelAuthOptions();
  const postLogoutTarget = options?.postLogoutRedirectUri || resolvePublicAppOrigin();

  if (!options) {
    return signOut({ redirectTo: "/" });
  }

  let discovery;
  try {
    discovery = await fetchZitadelDiscovery(options);
  } catch {
    return signOut({ redirectTo: postLogoutTarget });
  }
  const logoutUrl = discovery.end_session_endpoint
    ? new URL(discovery.end_session_endpoint)
    : new URL(postLogoutTarget);

  if (discovery.end_session_endpoint) {
    logoutUrl.searchParams.set("client_id", options.clientId);
    logoutUrl.searchParams.set("post_logout_redirect_uri", postLogoutTarget);
    const idToken = readZitadelServerIDToken(request.auth);
    if (idToken) {
      logoutUrl.searchParams.set("id_token_hint", idToken);
    }
  }

  return signOut({
    redirectTo:
      logoutUrl.origin === new URL(postLogoutTarget).origin
        ? `${logoutUrl.pathname}${logoutUrl.search}`
        : logoutUrl.toString(),
  });
});

export { authenticatedGET as GET };
