package membership

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"task-processor/internal/authidentity"
	domain "task-processor/internal/organization/membership"
)

type Writer struct{ read, write *Client }

func NewWriter(origin, readToken, writeToken, project string, client *http.Client) (*Writer, error) {
	read, err := NewClient(origin, readToken, project, client)
	if err != nil {
		return nil, err
	}
	write, err := NewClient(origin, writeToken, project, client)
	if err != nil {
		return nil, err
	}
	return &Writer{read, write}, nil
}

func (w *Writer) Write(ctx context.Context, op domain.Operation) (domain.Acknowledgment, error) {
	if w == nil || w.write == nil {
		return domain.Acknowledgment{}, domain.ErrUnavailable
	}
	if op.Phase != domain.PhaseDispatched || op.DispatchID == "" || op.Scope.ProjectID != w.write.project || !authidentity.IsBoundedIdentifier(op.Scope.OrganizationID) {
		return domain.Acknowledgment{}, domain.ErrInvalidRequest
	}
	prefix := "/zitadel.authorization.v2.AuthorizationService/"
	var path, dateField, id string
	var body any
	switch op.Step {
	case domain.StepUser:
		if op.Invitation == nil || !authidentity.IsBoundedIdentifier(op.TargetUserID) {
			return domain.Acknowledgment{}, domain.ErrInvalidRequest
		}
		path = "/v2/users/new"
		dateField = "creationDate"
		body = map[string]any{"organizationId": op.Scope.OrganizationID, "userId": op.TargetUserID, "username": op.Invitation.Email, "human": map[string]any{"profile": map[string]string{"givenName": op.Invitation.FirstName, "familyName": op.Invitation.LastName}, "email": map[string]any{"email": op.Invitation.Email, "sendCode": map[string]any{}}}}
	case domain.StepGrant:
		if !authidentity.IsBoundedIdentifier(op.TargetUserID) || op.Role == "" {
			return domain.Acknowledgment{}, domain.ErrInvalidRequest
		}
		path = prefix + "CreateAuthorization"
		dateField = "creationDate"
		body = map[string]any{"userId": op.TargetUserID, "projectId": op.Scope.ProjectID, "organizationId": op.Scope.OrganizationID, "roleKeys": []string{op.Role}}
	case domain.StepRole:
		if !authidentity.IsBoundedIdentifier(op.AuthorizationID) || op.Role == "" {
			return domain.Acknowledgment{}, domain.ErrInvalidRequest
		}
		path = prefix + "UpdateAuthorization"
		dateField = "changeDate"
		id = op.AuthorizationID
		body = map[string]any{"id": id, "roleKeys": []string{op.Role}}
	case domain.StepRemove:
		if !authidentity.IsBoundedIdentifier(op.AuthorizationID) {
			return domain.Acknowledgment{}, domain.ErrInvalidRequest
		}
		path = prefix + "DeleteAuthorization"
		dateField = "deletionDate"
		id = op.AuthorizationID
		body = map[string]string{"id": id}
	default:
		return domain.Acknowledgment{}, domain.ErrInvalidRequest
	}
	raw, err := w.write.requestJSON(ctx, http.MethodPost, path, body, false)
	if err != nil {
		return domain.Acknowledgment{}, err
	}
	var result map[string]json.RawMessage
	if json.Unmarshal(raw, &result) != nil {
		return domain.Acknowledgment{}, domain.ErrInvalidResponse
	}
	if id == "" {
		if json.Unmarshal(result["id"], &id) != nil || !authidentity.IsBoundedIdentifier(id) {
			return domain.Acknowledgment{}, domain.ErrInvalidResponse
		}
	}
	if op.Step == domain.StepUser && id != op.TargetUserID {
		return domain.Acknowledgment{}, domain.ErrInvalidResponse
	}
	var at string
	if json.Unmarshal(result[dateField], &at) != nil {
		return domain.Acknowledgment{}, domain.ErrInvalidResponse
	}
	if _, err := time.Parse(time.RFC3339Nano, at); err != nil {
		return domain.Acknowledgment{}, domain.ErrInvalidResponse
	}
	return domain.Acknowledgment{ID: id, At: at}, nil
}

func (w *Writer) ReadHuman(ctx context.Context, organization, id string) (domain.HumanIdentity, error) {
	if w == nil || w.read == nil {
		return domain.HumanIdentity{}, domain.ErrUnavailable
	}
	if !authidentity.IsBoundedIdentifier(id) {
		return domain.HumanIdentity{}, domain.ErrInvalidRequest
	}
	if err := w.read.requirePermission(ctx, organization, "user.read"); err != nil {
		return domain.HumanIdentity{}, err
	}
	raw, err := w.read.requestJSON(ctx, http.MethodGet, "/v2/users/"+id, nil, true)
	if err != nil {
		return domain.HumanIdentity{}, err
	}
	var response struct {
		User *struct {
			ID       string `json:"userId"`
			Username string `json:"username"`
			Details  struct {
				Owner string `json:"resourceOwner"`
			} `json:"details"`
			Machine json.RawMessage `json:"machine"`
			Human   *struct {
				Profile struct {
					First string `json:"givenName"`
					Last  string `json:"familyName"`
				} `json:"profile"`
				Email struct {
					Email string `json:"email"`
				} `json:"email"`
			} `json:"human"`
		} `json:"user"`
	}
	if json.Unmarshal(raw, &response) != nil || response.User == nil {
		return domain.HumanIdentity{}, domain.ErrInvalidResponse
	}
	user := response.User
	if user.ID != id || user.Details.Owner != organization || !authidentity.IsBoundedIdentifier(user.Details.Owner) || user.Human == nil || len(user.Machine) != 0 {
		return domain.HumanIdentity{}, domain.ErrInvalidResponse
	}
	h := domain.HumanIdentity{ID: id, OrganizationID: user.Details.Owner, Username: user.Username, Email: user.Human.Email.Email, FirstName: user.Human.Profile.First, LastName: user.Human.Profile.Last}
	for _, value := range []string{h.Username, h.Email, h.FirstName, h.LastName} {
		if value == "" || len(value) > 200 {
			return domain.HumanIdentity{}, domain.ErrInvalidResponse
		}
	}
	if err := w.read.requirePermission(ctx, organization, "user.read"); err != nil {
		return domain.HumanIdentity{}, err
	}
	return h, nil
}

func (c *Client) requestJSON(ctx context.Context, method, path string, body any, allowNotFound bool, organization ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil || len(data) > 16384 {
			return nil, domain.ErrInvalidRequest
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, c.origin+path, bytes.NewReader(data))
	if err != nil {
		return nil, domain.ErrUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Connect-Protocol-Version", "1")
	if len(organization) > 0 {
		if !authidentity.IsBoundedIdentifier(organization[0]) {
			return nil, domain.ErrInvalidRequest
		}
		request.Header.Set("x-zitadel-orgid", organization[0])
	}
	// No idempotency headers and no application retry: a mutation is dispatched once.
	response, err := c.http.Do(request)
	if err != nil {
		return nil, domain.ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound && allowNotFound {
		return nil, domain.ErrNotFound
	}
	if response.StatusCode != http.StatusOK {
		return nil, domain.ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 16385))
	if ctx.Err() != nil {
		return nil, domain.ErrUnavailable
	}
	if err != nil || len(raw) > 16384 || !json.Valid(raw) {
		return nil, domain.ErrInvalidResponse
	}
	return raw, nil
}
