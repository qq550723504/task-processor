-- Explicit isolated schema only. Not registered in production migrations.
CREATE TABLE product_title_proposals (
 org text NOT NULL, id text NOT NULL, owner text NOT NULL,
	state text NOT NULL CONSTRAINT ck_product_title_proposals_state
	 CHECK (state IN ('pending','accepted','rejected','applied')),
	payload bytea NOT NULL CONSTRAINT ck_product_title_proposals_payload_size
	 CHECK (octet_length(payload) <= 65536),
	CONSTRAINT ck_product_title_proposals_state_payload
	 CHECK (state = (convert_from(payload, 'UTF8')::jsonb ->> 'State')),
 PRIMARY KEY (org,id)
);
CREATE INDEX ix_product_title_proposals_actionable_admin
 ON product_title_proposals (org,id)
 WHERE state IN ('pending','accepted');
CREATE INDEX ix_product_title_proposals_actionable_owner
 ON product_title_proposals (org,owner,id)
 WHERE state IN ('pending','accepted');
CREATE TABLE product_title_operations (
 org text NOT NULL, actor text NOT NULL, operation_key text NOT NULL,
 fingerprint text NOT NULL, response bytea NOT NULL CHECK (octet_length(response) <= 65536),
 PRIMARY KEY (org,actor,operation_key)
);
