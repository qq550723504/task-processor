package readinessinspect

import "encoding/json"

func InputSchema() json.RawMessage {
	return json.RawMessage(`{
 "$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,
 "required":["product_key","catalog_version","target_platform"],"properties":{
 "product_key":{"type":"string","minLength":1,"maxLength":128},
 "catalog_version":{"type":"string","pattern":"^[1-9][0-9]{0,18}$"},
 "target_platform":{"type":"string","minLength":1,"maxLength":128}
 }}`)
}

func OutputSchema() json.RawMessage {
	return json.RawMessage(`{
 "$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,
 "required":["scope","status","product_key","product_version","publication_id","target_platform","reasons","missing_facts","asset_binding","marketplace","diagnostics"],
 "properties":{
 "scope":{"const":"product.inputs"},"status":{"enum":["ready","blocked","not_evaluated"]},
 "product_key":{"type":"string","minLength":1,"maxLength":128},
 "product_version":{"type":"string","pattern":"^[1-9][0-9]{0,18}$"},
 "publication_id":{"type":"string","minLength":1,"maxLength":128},
 "target_platform":{"type":"string","minLength":1,"maxLength":128},
 "input_rule_version":{"type":"string","minLength":1,"maxLength":128},
 "reasons":{"$ref":"#/$defs/strings"},"missing_facts":{"$ref":"#/$defs/strings"},
 "asset_binding":{"type":"object","additionalProperties":false,"required":["product_version","status","count"],"properties":{
 "product_version":{"type":"string","pattern":"^[1-9][0-9]{0,18}$"},"status":{"enum":["missing","exact"]},
 "count":{"type":"integer","minimum":0,"maximum":1024},"digest":{"type":"string","pattern":"^sha256:[a-f0-9]{64}$"}}},
 "marketplace":{"type":"object","additionalProperties":false,"required":["status","reasons"],"properties":{
 "status":{"const":"not_evaluated"},"reasons":{"$ref":"#/$defs/strings"}}},
 "diagnostics":{"type":"object","additionalProperties":false,"required":["needs_review","review_reasons","warnings"],"properties":{
 "needs_review":{"type":"boolean"},"review_reasons":{"$ref":"#/$defs/strings"},
 "warnings":{"type":"array","maxItems":128,"items":{"type":"object","additionalProperties":false,"required":["code","message"],"properties":{
 "code":{"type":"string"},"field":{"type":"string"},"message":{"type":"string"}}}}}}
 },"$defs":{"strings":{"type":"array","maxItems":128,"items":{"type":"string"}}}
 }`)
}
