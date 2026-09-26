CREATE TABLE public.subject_verification_applications (
 id varchar(128) PRIMARY KEY,
 scope varchar(128) NOT NULL,
 organization_id varchar(128) NOT NULL UNIQUE,
 actor_id varchar(128) NOT NULL,
 idempotency_key varchar(128) NOT NULL,
 input_digest varchar(128) NOT NULL,
 company_name varchar(256) NOT NULL,
 credit_code varchar(18) NOT NULL,
 phone_digest varchar(128) NOT NULL,
 masked_phone varchar(32) NOT NULL,
 correlation varchar(1000) NOT NULL,
 state varchar(32) NOT NULL CHECK (state IN ('OUTCOME_UNKNOWN','PENDING','VERIFIED')),
 encrypted_url bytea,
 provider_organization_id varchar(128) NOT NULL,
 provider_admin_id varchar(128) NOT NULL,
 created_at timestamptz NOT NULL,
 expires_at timestamptz NOT NULL,
 provider_verified_at timestamptz NOT NULL,
 observed_at timestamptz NOT NULL,
 UNIQUE (scope, correlation),
 CHECK (state <> 'VERIFIED' OR encrypted_url IS NULL)
);
CREATE TABLE public.subject_verification_messages (
 scope varchar(128) NOT NULL,
 message_id varchar(128) NOT NULL,
 digest varchar(128) NOT NULL,
 application_id varchar(128) NOT NULL REFERENCES public.subject_verification_applications(id),
 outcome varchar(32) NOT NULL,
 PRIMARY KEY (scope, message_id)
);
