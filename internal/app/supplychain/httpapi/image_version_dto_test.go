package supplychainhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/product/asset"
)

func TestSupplyImageVersionWirePreservesExactOwnerSelection(t *testing.T) {
	const version = uint64(9223372036854775807)
	const selection = `{"itemId":"source","originalPublicationId":"publication","originalSnapshotVersion":"9007199254740993","effectiveCatalogVersion":"9223372036854775807","targetPlatform":"shein"}`
	read := func(raw string, target any, key bool) error {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		if key {
			r.Header.Set("Idempotency-Key", "11111111-1111-4111-8111-111111111111")
		}
		_, err := supplyBody(r, target, key)
		return err
	}
	var inventoryBody supplyImageSourceBody
	require.NoError(t, read(selection, &inventoryBody, false))
	selected, err := inventoryBody.domain()
	require.NoError(t, err)
	require.Equal(t, uint64(9007199254740993), selected.OriginalSnapshotVersion)
	require.Equal(t, version, selected.EffectiveCatalogVersion)
	require.Equal(t, "publication", selected.OriginalPublicationID)
	var approval supplyImageApprovalBody
	require.NoError(t, read(`{"selection":`+selection+`,"images":[{"id":"image","role":"main"}],"approved":[]}`, &approval, true))
	approvedSource, err := approval.Selection.domain()
	require.NoError(t, err)
	require.Equal(t, selected, approvedSource)
	for _, invalid := range []string{`9223372036854775807`, `"0"`, `"01"`, `"+1"`, `"9223372036854775808"`} {
		var body supplyImageSourceBody
		err := read(strings.Replace(selection, `"9223372036854775807"`, invalid, 1), &body, false)
		if err == nil {
			_, err = body.domain()
		}
		require.Error(t, err)
	}
	owner := asset.ApprovedAssetInventory{Scope: asset.InventoryScope{TenantID: "org", ProductKey: "product", TargetPlatform: "shein", SourceSnapshotVersion: version}, Assets: []asset.ApprovedAsset{{ID: "approved", Role: asset.RoleMain, URL: "https://images.test/approved.png"}}}
	wire, err := json.Marshal(supplyImageInventory(owner))
	require.NoError(t, err)
	require.Contains(t, string(wire), `"source_snapshot_version":"9223372036854775807"`)
	require.Contains(t, string(wire), `"id":"approved"`)
	require.Equal(t, version, owner.Scope.SourceSnapshotVersion)
}
