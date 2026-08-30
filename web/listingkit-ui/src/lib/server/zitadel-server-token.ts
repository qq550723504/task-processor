type ZitadelServerSession = {
  accessToken?: unknown;
  idToken?: unknown;
};

export function readZitadelServerAccessToken(
  session: ZitadelServerSession | null | undefined,
) {
  return typeof session?.accessToken === "string" ? session.accessToken : "";
}

export function readZitadelServerIDToken(
  session: ZitadelServerSession | null | undefined,
) {
  return typeof session?.idToken === "string" ? session.idToken : "";
}
