// This deployment switch does not grant permission or prove channel qualification.
export function isEcoservicesAvailable(){return process.env.LISTINGKIT_ECOSERVICES_ENABLED==="true";}
export function isEcoservicesNonPaymentOnly(){return process.env.LISTINGKIT_ECOSERVICES_NON_PAYMENT_ONLY==="true";}
