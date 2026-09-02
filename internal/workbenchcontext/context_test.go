package workbenchcontext

import (
	"errors"
	"testing"

	"task-processor/internal/authidentity"
)

func TestSelectOrganizationUsesRequestedThenHomeThenSolePrecedence(t *testing.T) {
	grants := []authidentity.OrganizationGrant{
		{OrganizationID: "org-b", OrganizationName: " Beta ", ProjectID: "project-1", Roles: []string{"viewer"}},
		{OrganizationID: "org-a", OrganizationName: "Alpha", ProjectID: "project-1", Roles: []string{"admin"}},
	}

	testCases := []struct {
		name      string
		requested string
		home      string
		grants    []authidentity.OrganizationGrant
		want      string
		wantErr   error
	}{
		{name: "authorized requested organization wins", requested: "org-b", home: "org-a", grants: grants, want: "org-b"},
		{name: "authorized home organization wins without request", home: "org-a", grants: grants, want: "org-a"},
		{name: "sole organization is selected without request or home", grants: grants[:1], want: "org-b"},
		{name: "multiple organizations require selection", grants: grants, wantErr: ErrOrganizationSelectionRequired},
		{name: "unknown requested organization is denied instead of falling back", requested: "org-unknown", home: "org-a", grants: grants, wantErr: ErrOrganizationAccessDenied},
		{name: "no grants reports revoked access", home: "org-a", wantErr: ErrOrganizationAccessRevoked},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectOrganization(tt.requested, tt.home, tt.grants)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("SelectOrganization() error = %v, want errors.Is(..., %v)", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SelectOrganization() unexpected error = %v", err)
			}
			if got.OrganizationID != tt.want {
				t.Fatalf("SelectOrganization() organization = %q, want %q", got.OrganizationID, tt.want)
			}
		})
	}
}
