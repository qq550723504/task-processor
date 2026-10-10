package currentapplication

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	sigjson "sigs.k8s.io/json"
	coreconfig "task-processor/internal/core/config"
	"task-processor/internal/integration/sds"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
)

type podCredentialFile struct{ path string }

func (f podCredentialFile) Current(ctx context.Context) (sds.Credentials, error) {
	if ctx == nil || ctx.Err() != nil || !filepath.IsAbs(f.path) || coreconfig.VerifyPrivateFiles(ctx, []string{f.path}) != nil {
		return sds.Credentials{}, pod.ErrUnavailable
	}
	file, e := os.Open(f.path)
	if e != nil {
		return sds.Credentials{}, pod.ErrUnavailable
	}
	defer file.Close()
	raw, e := io.ReadAll(io.LimitReader(file, 65537))
	defer clear(raw)
	if e != nil || len(raw) > 65536 {
		return sds.Credentials{}, pod.ErrUnavailable
	}
	var value struct {
		Format       string          `json:"format"`
		BindingID    string          `json:"bindingId"`
		Revision     string          `json:"revision"`
		MerchantID   string          `json:"merchantId"`
		AccessToken  string          `json:"accessToken"`
		OutToken     string          `json:"outToken"`
		CapturedAt   string          `json:"capturedAt"`
		Source       json.RawMessage `json:"source"`
		Verification json.RawMessage `json:"verification"`
	}
	strict, e := sigjson.UnmarshalStrict(raw, &value, sigjson.DisallowDuplicateFields, sigjson.DisallowUnknownFields)
	if e != nil || len(strict) > 0 || value.Format != "sds-platform-session-v1" || !collection.ValidID(value.Revision) || ctx.Err() != nil {
		return sds.Credentials{}, pod.ErrUnavailable
	}
	return sds.Credentials{BindingID: value.BindingID, Revision: value.Revision, MerchantID: value.MerchantID, AccessToken: value.AccessToken, OutToken: value.OutToken}, nil
}

func PreparePODCredentials(ctx context.Context, path string, client *http.Client) (sds.CredentialSource, error) {
	source, e := sds.NewVerifiedCredentialSource(client, podCredentialFile{path})
	if e != nil {
		return nil, pod.ErrUnavailable
	}
	if _, e = source.Current(ctx); e != nil {
		return nil, pod.ErrUnavailable
	}
	return source, nil
}
