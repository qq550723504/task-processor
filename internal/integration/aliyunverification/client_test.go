package aliyunverification

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	domain "task-processor/internal/subjectverification"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestSDKFixedIdentityAndNoAutomaticRetry(t *testing.T) {
	c, err := NewClient("test-key", "test-secret", "https://app.example/workbench/account/profile/verification")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	c.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		n++
		for _, field := range []string{"CertName", "CertNo", "MetaInfo", "Mobile", "UserName", "IdentifyNum"} {
			if r.URL.Query().Has(field) {
				t.Fatalf("sensitive field %s entered request URL", field)
			}
		}
		var raw []byte
		if r.Body != nil {
			raw, _ = io.ReadAll(r.Body)
		}
		r.Form, _ = url.ParseQuery(string(raw))
		for k, v := range r.URL.Query() {
			r.Form[k] = v
		}
		action := r.Header.Get("x-acs-action")
		for k, v := range r.Header {
			if strings.EqualFold(k, "x-acs-action") && len(v) > 0 {
				action = v[0]
			}
		}
		if action == "" {
			action = r.Form.Get("Action")
		}
		body := `{"Code":"200","ResultObject":{"BizCode":"1"}}`
		switch action {
		case "Mobile3MetaSimpleVerify":
			if r.Form.Get("Mobile") != "13800000001" || r.Form.Get("IdentifyNum") != "110101199001010010" {
				t.Fatal("wrong phone identity")
			}
		case "InitFaceVerify":
			if r.Form.Get("CertName") != "测试姓名" || r.Form.Get("CertNo") != "110101199001010010" || r.Form.Get("RarelyCharacters") != "N" || r.Form.Get("ProductCode") != "ID_PRO" {
				t.Fatal("face identity not locked", r.Form)
			}
			body = `{"Code":"200","ResultObject":{"CertifyId":"cert1","CertifyUrl":"https://t.aliyun.com/test"}}`
		case "DescribeFaceVerify":
			if r.Form.Get("SceneId") != "123" || r.Form.Get("CertifyId") != "cert1" {
				t.Fatal("wrong stored binding")
			}
			body = `{"Code":"200","ResultObject":{"Passed":"T","SubCode":"200"}}`
		default:
			t.Fatal(action)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	identity := domain.PersonalIdentity{Name: "测试姓名", IDNumber: "110101199001010010", Phone: "13800000001"}
	if ok, err := c.VerifyPhone(context.Background(), identity); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if _, err = c.CreateFace(context.Background(), "f3915ce3-c613-46cf-a2f5-121f90c4d693", 123, identity, `{"deviceType":"pc"}`); err != nil {
		t.Fatal(err)
	}
	if got, err := c.QueryFace(context.Background(), 123, "cert1"); err != nil || got.Passed != "T" {
		t.Fatal(got, err)
	}
	if n != 3 {
		t.Fatal(n)
	}
	n = 0
	c.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		n++
		return &http.Response{StatusCode: 503, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"Code":"ServiceUnavailable","Message":"private text"}`))}, nil
	})}
	if _, err = c.VerifyPhone(context.Background(), identity); err == nil || strings.Contains(err.Error(), "private text") {
		t.Fatal("unredacted provider failure")
	}
	if n != 1 {
		t.Fatal("SDK retried paid call", n)
	}
	n = 0
	if _, err = c.CreateFace(context.Background(), "f3915ce3-c613-46cf-a2f5-121f90c4d693", 123, identity, `{"deviceType":"pc"}`); err == nil {
		t.Fatal("failed initialization accepted")
	}
	if n != 1 {
		t.Fatal("SDK retried initialization", n)
	}
}
func TestSDKContextCancelsTransport(t *testing.T) {
	c, _ := NewClient("test", "secret", "https://app.example/verification")
	c.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.QueryFace(ctx, 123, "cert")
	if err == nil || time.Since(start) > time.Second {
		t.Fatal("context did not reach transport")
	}
}
