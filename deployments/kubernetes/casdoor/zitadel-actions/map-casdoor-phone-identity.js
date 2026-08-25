// ZITADEL External Authentication action: map a verified Casdoor phone
// identity to a new ZITADEL user. Attached ONLY to
// External Authentication / Post Authentication; no Post Creation grant.
//
// Safety invariants enforced here and by static tests:
// - only the approved Casdoor issuers (production and staging) are accepted;
//   any missing or unexpected `iss` throws before any user is created
// - phone_verified claim must be true before any user is created
// - technical email on a .invalid domain, never the real phone number
// - no ListingKit role, tenant grant, or profile auto-update
const APPROVED_CASDOOR_ISSUERS = ["https://id.shuomiai.com", "https://id.staging.shuomiai.com"];
function mapCasdoorPhoneIdentity(ctx, api) {
  // ctx.claimsJSON is a serialized JSON string in the External
  // Authentication context, not a callable.
  const claims = JSON.parse(ctx.claimsJSON);
  if (!APPROVED_CASDOOR_ISSUERS.includes(claims.iss)) throw new Error("untrusted Casdoor issuer");
  if (typeof claims.sub !== "string" || !/^[A-Za-z0-9_-]{8,128}$/.test(claims.sub) || claims["https://shuomiai.com/claims/phone_verified"] !== true) throw new Error("verified Casdoor phone identity required");
  const id = claims.sub, alias = `casdoor-${id}@phone.id.shuomiai.invalid`;
  api.setFirstName(typeof claims.given_name === "string" && claims.given_name ? claims.given_name : "Phone");
  api.setLastName(typeof claims.family_name === "string" && claims.family_name ? claims.family_name : "User");
  api.setDisplayName(typeof claims.name === "string" && claims.name ? claims.name : "Phone user");
  api.setUsername(`casdoor-${id}`); api.setEmail(alias); api.setEmailVerified(true);
}
