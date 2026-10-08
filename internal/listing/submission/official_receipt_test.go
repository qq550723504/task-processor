package submission

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/collection"
	"task-processor/internal/storecenter"
	"testing"
	"time"
)

func TestOfficialCompletionBindsFullResultToExactCommittedAttemptAndPayload(t *testing.T) {
	scope := collection.Scope{"org-a", "actor-a", "member-a"}
	storeID := uuid.NewString()
	subject, err := ProductSubjectID("product-a", "shein-us")
	require.NoError(t, err)
	require.Equal(t, "product-us-"+digestExecutionParts("product-a", "shein-us"), subject)
	payload := model.PublishProduct{CategoryID: 123, ProductTypeID: 456, SourceSystem: "OpenAPI", SuitFlag: "0", SKCs: []model.ProductSKC{{SKUs: []model.ProductSKU{{SupplierSKU: "sku-a"}}}}, Sites: []model.SiteSelection{{MainSite: "shein", SubSites: []string{"shein-us"}}}}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	now := time.Now().UTC()
	id, err := uuid.NewV7()
	require.NoError(t, err)
	reservation, err := NewExecutionReservation(AcquireExecutionCommand{Scope: ExecutionScope{scope.OrganizationID}, IntentKey: "intent-a", Target: ExecutionTarget{"shein", storeID, subject}, Action: OfficialPublishAction, Payload: raw, ClaimOwnerID: "worker-a", Lease: time.Minute}, id.String(), "private-claim", now)
	require.NoError(t, err)
	attempt := reservation.Attempt
	attempt.FenceEpoch = 1
	claim := ExecutionClaim{Scope: ExecutionScope{scope.OrganizationID}, AttemptID: attempt.AttemptID, FenceEpoch: 1, OwnerID: attempt.ClaimOwnerID, Token: "private-claim"}
	binding := storecenter.ProductMerchantBinding{OrganizationID: scope.OrganizationID, StoreID: storeID, Site: "shein-us", StoreVersion: 1, ConnectionRevision: 1, ApplicationRevision: "v1:self_operated", ApplicationID: "app-a", ApplicationType: storecenter.ApplicationSelfOperated, SupplierIdentityHash: collection.Digest("supplier"), ServiceExpiresAt: now.Add(time.Hour)}
	result := model.PublishResult{SPUName: "spu-a", SKCs: []model.PublishedSKC{{SKCName: "skc-a", SKUs: []model.PublishedSKU{{SupplierSKU: "sku-a", SKUCode: "code-a"}}}}, ResponseHash: collection.Digest("response")}
	proof, err := NewPublishCompletion(scope, uuid.NewString(), "product-a", binding, attempt, claim, payload, result)
	require.NoError(t, err)
	gotClaim, receipt, err := proof.Read(context.Background())
	require.NoError(t, err)
	require.Equal(t, claim, gotClaim)
	require.Equal(t, attempt.PayloadFingerprint, receipt.PayloadFingerprint)
	require.Equal(t, result.ResponseHash, receipt.ResponseHash)
	encoded, err := json.Marshal(proof)
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(encoded))
	result.SKCs[0].SKUs[0].SupplierSKU = "another-sku"
	_, err = NewPublishCompletion(scope, uuid.NewString(), "product-a", binding, attempt, claim, payload, result)
	require.ErrorIs(t, err, ErrExecutionEvidenceRequired)
	payload.CategoryID++
	receipt.Product.ResponseHash = receipt.ResponseHash
	_, err = NewPublishCompletion(scope, uuid.NewString(), "product-a", binding, attempt, claim, payload, receipt.Product)
	require.ErrorIs(t, err, ErrExecutionEvidenceRequired, "a changed payload cannot finalize the original permit")
	_, _, err = (OfficialCompletion{}).Read(context.Background())
	require.ErrorIs(t, err, ErrExecutionEvidenceRequired)
}
