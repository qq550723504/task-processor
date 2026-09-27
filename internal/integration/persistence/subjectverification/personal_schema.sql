CREATE TABLE public.personal_verification_applications (
 id uuid PRIMARY KEY,
 user_id varchar(128) NOT NULL,
 scope varchar(128) NOT NULL,
 idempotency_key uuid NOT NULL,
 input_digest varchar(128) NOT NULL,
 identity_digest varchar(128) NOT NULL,
 phone_digest varchar(128) NOT NULL,
 masked_phone varchar(32) NOT NULL,
 state varchar(32) NOT NULL CHECK (state IN ('OUTCOME_UNKNOWN','PENDING','VERIFIED','REJECTED')),
 provider_scene_id bigint NOT NULL CHECK (provider_scene_id > 0),
 provider_certify_id varchar(256) NOT NULL,
 encrypted_url bytea,
 created_at timestamptz NOT NULL,
 expires_at timestamptz NOT NULL,
 phone_verified_at timestamptz NOT NULL,
 verified_at timestamptz NOT NULL,
 refresh_after timestamptz NOT NULL,
 refresh_token varchar(36) NOT NULL,
 UNIQUE (user_id, idempotency_key),
 CHECK (state NOT IN ('PENDING','VERIFIED') OR (provider_certify_id <> '' AND phone_verified_at > '1970-01-01')),
 CHECK (state <> 'VERIFIED' OR (encrypted_url IS NULL AND verified_at > '1970-01-01'))
);
CREATE INDEX personal_verification_user_created ON public.personal_verification_applications(user_id, created_at DESC, id DESC);
