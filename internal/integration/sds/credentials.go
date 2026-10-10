package sds

import (
	"context"
	"net/http"
	sigjson "sigs.k8s.io/json"
	"task-processor/internal/product/pod"
	"time"
)

type VerifiedCredentialSource struct {
	read   *ReadbackClient
	loader CredentialSource
}

// Each current credential revision is checked using the authenticated merchant
// endpoint. A declared merchant ID alone is never execution authority.
func NewVerifiedCredentialSource(h *http.Client, loader CredentialSource) (*VerifiedCredentialSource, error) {
	r, e := NewReadbackClient(h, loader)
	if e != nil {
		return nil, e
	}
	return &VerifiedCredentialSource{r, loader}, nil
}
func (s *VerifiedCredentialSource) Current(ctx context.Context) (Credentials, error) {
	if ctx == nil || ctx.Err() != nil || s == nil || s.loader == nil {
		return Credentials{}, pod.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	c, e := s.loader.Current(ctx)
	if e != nil || !validCredentials(c) {
		return Credentials{}, pod.ErrUnavailable
	}
	// Readback's fixed-host HTTPS client rejects redirects and cookies, bounds
	// response bytes, and returns only domain errors without credential payloads.
	var raw merchantIdentityDTO
	if s.read.get(ctx, c, "mapi.sdspod.com", "/merchants/"+c.MerchantID+"/setMeals", nil, &raw) != nil || string(raw.ID) != c.MerchantID || ctx.Err() != nil {
		return Credentials{}, pod.ErrUnavailable
	}
	return c, nil
}

type merchantIdentityDTO struct {
	ID remoteID `json:"id"`
}

func (d *merchantIdentityDTO) UnmarshalJSON(raw []byte) error {
	// Ignore provider metadata but reject duplicate fields; only identity is used.
	type plain merchantIdentityDTO
	var value plain
	errs, e := sigjson.UnmarshalStrict(raw, &value, sigjson.DisallowDuplicateFields)
	if e != nil || len(errs) > 0 {
		return pod.ErrUnavailable
	}
	*d = merchantIdentityDTO(value)
	return nil
}
