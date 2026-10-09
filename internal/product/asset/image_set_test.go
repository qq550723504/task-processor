package asset

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func imageSetContractFixture() ApprovalCommit {
	hash := strings.Repeat("a", 64)
	commit := ApprovalCommit{TenantID: "org", ProductKey: "product", TargetPlatform: "product", ActionID: "select", SourceSnapshotVersion: 1,
		ImageSet: &ImageSetSelection{Schema: ImageSetSelectionSchema, RequestDigest: hash, Source: SourceSelectionRequest{ItemID: "item", OriginalPublicationID: "publication", OriginalSnapshotVersion: 1, EffectiveCatalogVersion: 1, TargetPlatform: "product"}, Results: []ImageSetResultBinding{{RunID: "run", PlanRevision: 1, ResultDigest: hash}}},
		Assets:   []ApprovedAsset{{ID: "selected", RunID: "run", PlanRevision: 1, SlotID: "slot", Attempt: 1, Role: RoleGallery, URL: "https://images.example.org/output.png", Width: 1024, Height: 1024, Presentation: &ImagePresentation{Group: "detail", Order: 1}, GenerationEvidence: &GenerationEvidence{IntentID: hash, Fingerprint: hash, SettlementProofDigest: hash, ArtifactHash: hash}}}}
	commit.ImageSet.Digest = ImageSetSelectionDigest(commit)
	return commit
}

func TestImageSetCommitBindsSelectedOriginsToTheExactResultAndSource(t *testing.T) {
	require.NoError(t, ValidateApprovalCommit(imageSetContractFixture()))
	for _, mode := range []string{"missing_result", "extraneous_result", "foreign_publication", "mixed_source_evidence", "generic_with_target", "platform_without_target", "platform_target_missing_context", "platform_site_drift"} {
		t.Run(mode, func(t *testing.T) {
			commit := imageSetContractFixture()
			switch mode {
			case "missing_result":
				commit.ImageSet.Results = nil
			case "extraneous_result":
				commit.ImageSet.Results = append(commit.ImageSet.Results, ImageSetResultBinding{RunID: "other", PlanRevision: 1, ResultDigest: strings.Repeat("b", 64)})
			case "foreign_publication", "mixed_source_evidence":
				image := &commit.Assets[0]
				image.RunID, image.SlotID, image.PlanRevision, image.Attempt = "", "", 0, 0
				image.SourceAssetID = "original"
				image.SourceApproval = &SourceApprovalProvenance{OriginalPublicationID: "publication", OriginalSnapshotVersion: 1, ActorID: "actor", MemberID: "member", ReferenceHash: ReferenceHash(image.SourceAssetID, image.URL)}
				commit.ImageSet.Results = nil
				if mode == "foreign_publication" {
					image.GenerationEvidence = nil
					image.SourceApproval.OriginalPublicationID = "foreign"
				}
			case "generic_with_target":
				commit.ImageSet.Target = &ImageSetTarget{}
			default:
				commit.TargetPlatform, commit.ImageSet.Source.TargetPlatform = "shein", "shein"
				commit.ImageSet.RequirementDigest = strings.Repeat("b", 64)
				commit.Assets[0].OfficialPlacement = &ImageOfficialPlacement{Group: "skc", Type: 1, Sort: 1, Site: "shein-us"}
				if mode == "platform_target_missing_context" {
					commit.ImageSet.Target = &ImageSetTarget{StoreID: "store"}
				}
				if mode == "platform_site_drift" {
					commit.ImageSet.Target = &ImageSetTarget{StoreID: "store", Site: "shein-fr", ApplicationID: "application", ApplicationMode: "fully_managed", CategoryID: 1, ProductTypeID: 2, AttributesDigest: strings.Repeat("c", 64), VariantsDigest: strings.Repeat("d", 64)}
				}
			}
			commit.ImageSet.Digest = ImageSetSelectionDigest(commit)
			require.ErrorIs(t, ValidateApprovalCommit(commit), ErrInvalidApproval)
		})
	}
}
