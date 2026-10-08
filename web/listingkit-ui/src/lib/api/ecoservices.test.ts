import {describe,it,expect} from "vitest";
import {ecoCheckoutSchema} from "./ecoservices";

describe("service checkout QR contract",()=>{
 it("accepts both original Native URL variants and rejects other destinations",()=>{
  const orderId="7d25708b-b8cb-4e89-ac61-b5da1b4f0b08";
  for(const codeUrl of ["weixin://wxpay/bizpayurl?pr=original","weixin://wxpay/bizpayurl/up?pr=original"])
   expect(ecoCheckoutSchema.safeParse({orderId,codeUrl}).success,codeUrl).toBe(true);
  for(const codeUrl of ["https://attacker.example/qr","weixin://wxpay/bizpayurl/other?pr=x","weixin://wxpay.attacker.example/bizpayurl?pr=x","weixin://user@wxpay/bizpayurl?pr=x","weixin://wxpay/bizpayurl/up?pr=x#fragment"])
   expect(ecoCheckoutSchema.safeParse({orderId,codeUrl}).success,codeUrl).toBe(false);
 });
});
