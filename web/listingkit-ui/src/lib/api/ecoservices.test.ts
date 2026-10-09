import {describe,it,expect} from "vitest";
import {ecoCheckoutSchema,ecoFinancialSchema,ecoInputs} from "./ecoservices";

it("requires exact canonical chargeback facts and never invents zero for missing data",()=>{
 const facts={grossMinor:"101",refundedMinor:"2",platformMinor:"7",providerMinor:"72",sharedMinor:"10",returnedMinor:"3",releasedMinor:"91",channelFeeMinor:"0",channelFeeObserved:false,reconciliationReason:""};
 for(const chargedBackMinor of ["0","20","101","9007199254740993","9223372036854775807"]){const result=ecoFinancialSchema.safeParse({...facts,chargedBackMinor});expect(result.success).toBe(true);if(result.success)expect(result.data).toMatchObject({chargedBackMinor,refundedMinor:"2"})}
 expect(ecoFinancialSchema.safeParse(facts).success).toBe(false);
 for(const chargedBackMinor of [-1,20,null,"-1","020","2.0","9223372036854775808"])
  expect(ecoFinancialSchema.safeParse({...facts,chargedBackMinor}).success).toBe(false);
});

it("rejects malformed sibling version strings without throwing or changing the positive int64 contract",()=>{
 for(const deliveryVersion of ["2.0","n/a","","-1","01","0","9223372036854775808"])
  expect(ecoInputs.accept.safeParse({deliveryVersion}).success).toBe(false);
 expect(ecoInputs.accept.parse({deliveryVersion:"9223372036854775807"})).toEqual({deliveryVersion:"9223372036854775807"});
});

describe("service checkout QR contract",()=>{
 it("accepts both original Native URL variants and rejects other destinations",()=>{
  const orderId="7d25708b-b8cb-4e89-ac61-b5da1b4f0b08";
  for(const codeUrl of ["weixin://wxpay/bizpayurl?pr=original","weixin://wxpay/bizpayurl/up?pr=original"])
   expect(ecoCheckoutSchema.safeParse({orderId,codeUrl}).success,codeUrl).toBe(true);
  for(const codeUrl of ["https://attacker.example/qr","weixin://wxpay/bizpayurl/other?pr=x","weixin://wxpay.attacker.example/bizpayurl?pr=x","weixin://user@wxpay/bizpayurl?pr=x","weixin://wxpay/bizpayurl/up?pr=x#fragment"])
   expect(ecoCheckoutSchema.safeParse({orderId,codeUrl}).success,codeUrl).toBe(false);
 });
});
