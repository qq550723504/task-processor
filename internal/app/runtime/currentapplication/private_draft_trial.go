package currentapplication

import (
	"errors"
	"task-processor/internal/authidentity"
	"task-processor/internal/product/collection"
)

type PrivateDraftTrialConfig struct {
	Acknowledgment string `json:"acknowledgment"`
	OrganizationID string `json:"organizationId"`
	ActorID        string `json:"actorId"`
	MemberID       string `json:"memberId"`
	StoreID        string `json:"storeId"`
}

func (c *Config) validatePrivateDraftTrial() error {
	t := c.PrivateDraftTrial
	if t == nil {
		return nil
	}
	if t.Acknowledgment != "ISOLATED_OFFLINE_DRAFT_TRIAL_ONLY" || !authidentity.IsBoundedIdentifier(t.OrganizationID) || !authidentity.IsBoundedIdentifier(t.ActorID) || !authidentity.IsBoundedIdentifier(t.MemberID) || !collection.ValidID(t.StoreID) {
		return errors.New("private draft trial requires explicit acknowledgment and one complete synthetic scope/store")
	}
	if c.SupplyChain != nil || c.LocalTrial != nil || c.ImageAgent != nil || c.ProductAgent != nil || c.BrowserCollector != nil || !c.ProductCollections || c.ProductAcquisitionDatabase == nil || c.AgentCustomizationDatabase == nil || c.StoreCenter == nil || !c.StoreCenter.Enabled || len(c.StoreCenter.OfficialApplications) != 0 || c.Identity.TenantDirectoryToken == "" {
		return errors.New("private draft trial requires current Product/Store/customization owners and excludes official execution")
	}
	issuer, err := validateLoopbackURL("issuerURL", c.Identity.IssuerURL)
	if err != nil || issuer.Scheme != "https" {
		return errors.New("private draft trial requires local HTTPS identity")
	}
	return nil
}
