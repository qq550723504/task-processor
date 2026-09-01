// Package commercetool defines governed tool metadata and its static invariants.
//
// Tool schemas are fixed to JSON Schema Draft 2020-12, form one resource, and
// may reference only fragments in their own document. Construction audits both
// semantic subschemas and every reachable local reference target, including
// targets stored under annotation keywords. It fails closed when a schema can
// admit an object without proving that reserved authority fields are excluded:
// object-capable schemas must set additionalProperties to false, arrays must
// constrain items, and schemas without an explicit type must delegate through a
// local reference/composition or enumerate finite const/enum values. Reserved
// Principal, call, Agent, Tool, and permission fields are rejected throughout
// the reachable schema graph in both input and output schemas.
package commercetool
