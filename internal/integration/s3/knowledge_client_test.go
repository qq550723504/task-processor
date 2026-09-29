package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestKnowledgeClientNeverFollowsObjectRedirects(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		for _, method := range []string{http.MethodHead, http.MethodGet, http.MethodPut} {
			t.Run(fmt.Sprintf("%d/%s", status, method), func(t *testing.T) {
				var forwarded, signed, sourceCalls atomic.Int32
				target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					forwarded.Add(1)
					if r.Header.Get("Authorization") != "" {
						signed.Add(1)
					}
					w.Header().Set("Content-Length", "21")
					_, _ = io.WriteString(w, "synthetic private doc")
				}))
				defer target.Close()
				source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					sourceCalls.Add(1)
					w.Header().Set("Location", target.URL+r.URL.Path)
					w.WriteHeader(status)
				}))
				defer source.Close()
				client, err := NewKnowledgeClient(ClientConfig{Region: "us-east-1", Endpoint: source.URL, AccessKeyID: "synthetic-access", SecretAccessKey: "synthetic-secret", UsePathStyle: true})
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				switch method {
				case http.MethodHead:
					_, err = client.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String("knowledge"), Key: aws.String("knowledge/test")})
				case http.MethodGet:
					var result *awss3.GetObjectOutput
					result, err = client.GetObject(ctx, &awss3.GetObjectInput{Bucket: aws.String("knowledge"), Key: aws.String("knowledge/test")})
					if result != nil && result.Body != nil {
						_, _ = io.Copy(io.Discard, result.Body)
						_ = result.Body.Close()
					}
				case http.MethodPut:
					_, err = client.PutObject(ctx, &awss3.PutObjectInput{Bucket: aws.String("knowledge"), Key: aws.String("knowledge/test"), Body: bytes.NewReader([]byte("synthetic private doc"))})
				}
				if sourceCalls.Load() == 0 {
					t.Fatal("test did not reach the configured object endpoint")
				}
				if forwarded.Load() != 0 || err == nil {
					t.Fatalf("redirect must fail at the configured endpoint: forwarded=%d signed=%d err=%v", forwarded.Load(), signed.Load(), err)
				}
			})
		}
	}
}
