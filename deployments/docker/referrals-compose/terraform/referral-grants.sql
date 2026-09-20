REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC;
REVOKE CREATE, TEMP ON DATABASE referrals FROM PUBLIC;
GRANT CONNECT ON DATABASE referrals TO referral_runtime;
GRANT USAGE ON SCHEMA public TO referral_runtime;
GRANT SELECT, INSERT ON TABLE public.referral_codes, public.registration_intents, public.referral_relations, public.referral_receipts, public.registration_admission_buckets TO referral_runtime;
GRANT UPDATE (state, ciphertext, lease_until) ON TABLE public.registration_intents TO referral_runtime;
GRANT UPDATE, DELETE ON TABLE public.registration_admission_buckets TO referral_runtime;
ALTER ROLE referral_runtime SET statement_timeout='10s';
