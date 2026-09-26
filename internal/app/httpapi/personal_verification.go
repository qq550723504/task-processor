package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"task-processor/internal/authidentity"
	zitadel "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/core/config"
	"task-processor/internal/integration/aliyunverification"
	store "task-processor/internal/integration/persistence/subjectverification"
	domain "task-processor/internal/subjectverification"
	verificationhttp "task-processor/internal/subjectverification/httpapi"
)

func buildPersonalVerification(ctx context.Context, db *gorm.DB, cfg *config.Config) (verificationhttp.PersonalHandler, error) {
	h := verificationhttp.PersonalHandler{}
	path := strings.TrimSpace(os.Getenv("ALIYUN_PERSONAL_VERIFICATION_CONFIG_FILE"))
	if path == "" {
		return h, nil
	}
	invalid := errors.New("personal verification private configuration invalid")
	if config.VerifyPrivateFiles(ctx, []string{path}) != nil {
		return h, invalid
	}
	f, err := os.Open(path)
	if err != nil {
		return h, invalid
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 8193))
	if err != nil || len(raw) > 8192 {
		return h, invalid
	}
	var values struct {
		Scope, AccessKeyID, AccessKeySecret, ReturnURL, DataEncryptionKey string
		SceneID                                                           int64
		TotalLimit, DailyLimit, MinIntervalSeconds                        *int
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	var extra any
	if d.Decode(&values) != nil || d.Decode(&extra) != io.EOF {
		return h, invalid
	}
	u, err := url.Parse(values.ReturnURL)
	if err != nil || !domain.ValidPersonalURL(values.ReturnURL) || u.Path != "/workbench/account/profile/verification" || u.RawQuery != "" || u.ForceQuery || !authidentity.IsBoundedIdentifier(values.Scope) || values.SceneID <= 0 {
		return h, invalid
	}
	limits := domain.DefaultPersonalLimits()
	if values.TotalLimit != nil {
		limits.Total = *values.TotalLimit
	}
	if values.DailyLimit != nil {
		limits.Daily = *values.DailyLimit
	}
	if values.MinIntervalSeconds != nil {
		limits.IntervalSeconds = *values.MinIntervalSeconds
	}
	if !limits.Valid() {
		return h, invalid
	}
	key, err := base64.StdEncoding.DecodeString(values.DataEncryptionKey)
	if err != nil {
		return h, invalid
	}
	protection, err := store.NewProtection(key)
	if err != nil {
		return h, invalid
	}
	provider, err := aliyunverification.NewClient(values.AccessKeyID, values.AccessKeySecret, values.ReturnURL)
	if err != nil {
		return h, invalid
	}
	repository, err := store.NewPersonalRepository(ctx, db)
	if err != nil {
		return h, err
	}
	h.Service = &domain.PersonalService{Store: repository, Provider: provider, Protection: protection, Scope: values.Scope, SceneID: values.SceneID, Limits: limits}
	h.Profile = zitadel.NewUserInfoClient(cfg.ListingKit.Zitadel.IssuerURL, &http.Client{})
	return h, nil
}
