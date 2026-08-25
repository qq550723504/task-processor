function mapCasdoorPhoneIdentity(ctx, api) {
  const claims = ctx.claimsJSON();
  if (claims.iss !== "https://id.staging.shuomiai.com") return;
  if (
    typeof claims.sub !== "string" ||
    !/^[A-Za-z0-9_-]{8,128}$/.test(claims.sub) ||
    claims["https://shuomiai.com/claims/phone_verified"] !== true
  ) {
    throw new Error("verified Casdoor phone identity required");
  }

  const id = claims.sub;
  const alias = `casdoor-${id}@phone.id.shuomiai.invalid`;
  api.setFirstName(
    typeof claims.given_name === "string" && claims.given_name
      ? claims.given_name
      : "Phone"
  );
  api.setLastName(
    typeof claims.family_name === "string" && claims.family_name
      ? claims.family_name
      : "User"
  );
  api.setDisplayName(
    typeof claims.name === "string" && claims.name
      ? claims.name
      : "Phone user"
  );
  api.setUsername(`casdoor-${id}`);
  api.setEmail(alias);
  api.setEmailVerified(true);
}
