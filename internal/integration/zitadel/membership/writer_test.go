package membership

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"strings"
	"testing"

	domain "task-processor/internal/organization/membership"
)

func TestWriterUsesFixedIdentityAndExactAuthorizationEndpoints(t *testing.T) {
	steps := []domain.OperationStep{domain.StepUser, domain.StepGrant, domain.StepRole, domain.StepRemove}
	paths := []string{"/v2/users/new", "/zitadel.authorization.v2.AuthorizationService/CreateAuthorization", "/zitadel.authorization.v2.AuthorizationService/UpdateAuthorization", "/zitadel.authorization.v2.AuthorizationService/DeleteAuthorization"}
	for i, step := range steps {
		t.Run(string(step), func(t *testing.T) {
			calls := 0
			server := authorizedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != paths[i] || r.Header.Get("Authorization") != "Bearer write-only" {
					t.Errorf("unexpected route/header %s %s", r.Method, r.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if step == domain.StepUser {
					if body["userId"] != "fixed-user" || body["organizationId"] != "org" || body["username"] != "new@example.com" || body["human"] == nil {
						t.Errorf("create=%v", body)
					}
				} else if step == domain.StepGrant {
					if body["userId"] != "fixed-user" || body["projectId"] != "project" || body["organizationId"] != "org" {
						t.Errorf("grant=%v", body)
					}
				} else if body["id"] != "grant" {
					t.Errorf("wrong id=%v", body)
				}
				if step == domain.StepRemove && len(body) != 1 {
					t.Errorf("remove extra fields=%v", body)
				}
				fmt.Fprint(w, `{"id":"fixed-user","creationDate":"2026-09-12T00:00:00Z","changeDate":"2026-09-12T00:00:00Z","deletionDate":"2026-09-12T00:00:00Z"}`)
			}))
			defer server.Close()
			client, err := NewWriter(server.URL, "read-only", "write-only", "project", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			ack, err := client.Write(context.Background(), domain.Operation{Scope: domain.OperationScope{ProjectID: "project", OrganizationID: "org"}, Step: step, Phase: domain.PhaseDispatched, DispatchID: "dispatch", TargetUserID: "fixed-user", AuthorizationID: "grant", Role: "listingkit_viewer", Invitation: &domain.Invitation{Email: "new@example.com", FirstName: "New", LastName: "Member"}})
			if err != nil || ack.At == "" || calls != 1 {
				t.Fatalf("ack=%+v err=%v calls=%d", ack, err, calls)
			}
		})
	}
}

func TestWriterErrorsAreUnknownAndNeverRetried(t *testing.T) {
	for _, status := range []int{400, 403, 404, 409, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			server := authorizedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(status)
				fmt.Fprint(w, "secret provider detail")
			}))
			defer server.Close()
			client, _ := NewWriter(server.URL, "read", "write", "project", server.Client())
			_, err := client.Write(context.Background(), domain.Operation{Scope: domain.OperationScope{ProjectID: "project", OrganizationID: "org"}, Phase: domain.PhaseDispatched, DispatchID: "dispatch", Step: domain.StepRemove, AuthorizationID: "grant"})
			if err == nil || strings.Contains(err.Error(), "secret") || calls != 1 {
				t.Fatalf("err=%v calls=%d", err, calls)
			}
		})
	}
}

func TestReadHumanRequiresExactIDHumanAndOwner(t *testing.T) {
	server := authorizedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v2/users/fixed-user" || r.Header.Get("Authorization") != "Bearer read" {
			t.Error("wrong user read")
		}
		fmt.Fprint(w, `{"user":{"userId":"fixed-user","username":"new@example.com","details":{"resourceOwner":"org"},"human":{"profile":{"givenName":"New","familyName":"Member"},"email":{"email":"new@example.com"}}}}`)
	}))
	defer server.Close()
	client, _ := NewWriter(server.URL, "read", "write", "project", server.Client())
	human, err := client.ReadHuman(context.Background(), "org", "fixed-user")
	if err != nil || human.OrganizationID != "org" || human.FirstName != "New" {
		t.Fatalf("human=%+v %v", human, err)
	}
}
