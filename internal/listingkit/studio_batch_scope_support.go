package listingkit

func applyStudioBatchDefaultScope(existingTenantID, existingUserID string, tenantID *string, userID *string) {
	if tenantID != nil && *tenantID == "" {
		*tenantID = existingTenantID
	}
	if userID != nil && *userID == "" {
		*userID = existingUserID
	}
}

func requireStudioBatchOwnership(existing, incoming string) (string, error) {
	if incoming == "" {
		return existing, nil
	}
	if incoming != existing {
		return "", ErrStudioBatchOwnershipConflict
	}
	return incoming, nil
}

func resolveStudioBatchItemOwnership(existing StudioBatchItemRecord, row *StudioBatchItemRecord) error {
	var err error
	row.BatchID, err = requireStudioBatchOwnership(existing.BatchID, row.BatchID)
	if err != nil {
		return err
	}
	row.TenantID, err = requireStudioBatchOwnership(existing.TenantID, row.TenantID)
	if err != nil {
		return err
	}
	row.UserID, err = requireStudioBatchOwnership(existing.UserID, row.UserID)
	return err
}

func resolveStudioGenerationAttemptOwnership(existing StudioGenerationAttemptRecord, row *StudioGenerationAttemptRecord) error {
	var err error
	row.BatchID, err = requireStudioBatchOwnership(existing.BatchID, row.BatchID)
	if err != nil {
		return err
	}
	row.ItemID, err = requireStudioBatchOwnership(existing.ItemID, row.ItemID)
	if err != nil {
		return err
	}
	row.TenantID, err = requireStudioBatchOwnership(existing.TenantID, row.TenantID)
	if err != nil {
		return err
	}
	row.UserID, err = requireStudioBatchOwnership(existing.UserID, row.UserID)
	return err
}

func resolveStudioMaterializedDesignOwnership(existing StudioMaterializedDesignRecord, row *StudioMaterializedDesignRecord) error {
	var err error
	row.BatchID, err = requireStudioBatchOwnership(existing.BatchID, row.BatchID)
	if err != nil {
		return err
	}
	row.ItemID, err = requireStudioBatchOwnership(existing.ItemID, row.ItemID)
	if err != nil {
		return err
	}
	row.SourceAttemptID, err = requireStudioBatchOwnership(existing.SourceAttemptID, row.SourceAttemptID)
	if err != nil {
		return err
	}
	row.TenantID, err = requireStudioBatchOwnership(existing.TenantID, row.TenantID)
	if err != nil {
		return err
	}
	row.UserID, err = requireStudioBatchOwnership(existing.UserID, row.UserID)
	return err
}
