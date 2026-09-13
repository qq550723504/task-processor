//go:build integration

package referral

import (
	"context"
	"testing"
)

func TestRuntimeCannotMutateImmutableFactsOrProof(t *testing.T) {
	owner, runtime := ownedDatabase(t)
	for _, sql := range []string{
		"UPDATE public.referral_relations SET referrer='forged'",
		"DELETE FROM public.referral_receipts",
		"TRUNCATE public.referral_relations",
		"UPDATE public.registration_intents SET fingerprint='forged'",
		"UPDATE public.registration_intents SET subject='forged'",
		"DELETE FROM public.registration_intents",
		"CREATE TABLE public.forbidden(id integer)",
	} {
		if err := runtime.Exec(sql).Error; err == nil {
			t.Fatalf("runtime accepted forbidden SQL %s", sql)
		}
	}
	if err := owner.Exec("GRANT UPDATE(referrer) ON public.referral_relations TO referral_runtime").Error; err != nil {
		t.Fatal(err)
	}
	if err := VerifyPermissions(context.Background(), runtime); err == nil {
		t.Fatal("privilege drift must prevent startup")
	}
}
