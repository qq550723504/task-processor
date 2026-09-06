-- Explicit isolated schema only. Not registered in production migrations.
CREATE TABLE product_title_proposals (
 org text NOT NULL, id text NOT NULL, owner text NOT NULL,
 payload bytea NOT NULL CHECK (octet_length(payload) <= 65536),
 PRIMARY KEY (org,id)
);
CREATE TABLE product_title_operations (
 org text NOT NULL, actor text NOT NULL, operation_key text NOT NULL,
 fingerprint text NOT NULL, response bytea NOT NULL CHECK (octet_length(response) <= 65536),
 PRIMARY KEY (org,actor,operation_key)
);
