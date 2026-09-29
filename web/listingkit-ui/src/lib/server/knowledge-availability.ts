// Deployment availability is independent of the live Go authorization decision.
export function isKnowledgeAvailable(){return process.env.LISTINGKIT_KNOWLEDGE_ENABLED==="true";}
