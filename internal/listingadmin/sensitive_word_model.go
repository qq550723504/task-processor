package listingadmin

import "strings"

func (r listingSensitiveWord) toSensitiveWord() SensitiveWord {
	return SensitiveWord{
		ID:          r.ID,
		TenantID:    r.TenantID,
		Word:        r.Word,
		Language:    r.Language,
		Tags:        r.Tags,
		Level:       r.Level,
		ReplaceWord: r.ReplaceWord,
		Remark:      r.Remark,
		Status:      r.Status,
		CreateTime:  r.CreateTime,
		UpdateTime:  r.UpdateTime,
	}
}

func listingSensitiveWordFromSensitiveWord(word *SensitiveWord) listingSensitiveWord {
	if word == nil {
		return listingSensitiveWord{}
	}
	return listingSensitiveWord{
		ID:          word.ID,
		TenantID:    word.TenantID,
		Word:        strings.TrimSpace(word.Word),
		Language:    strings.TrimSpace(word.Language),
		Tags:        strings.TrimSpace(word.Tags),
		Level:       word.Level,
		ReplaceWord: strings.TrimSpace(word.ReplaceWord),
		Remark:      strings.TrimSpace(word.Remark),
		Status:      word.Status,
	}
}

func applySensitiveWordDefaults(row *listingSensitiveWord) {
	if row.Language == "" {
		row.Language = "en"
	}
	if row.Level <= 0 {
		row.Level = 1
	}
}

func applySensitiveWordAuditFields(row *listingSensitiveWord, userID string, includeCreate bool) {
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return
	}
	row.OwnerUserID = trimmedUserID
	row.Updater = trimmedUserID
	row.UpdatedBy = trimmedUserID
	if includeCreate {
		row.Creator = trimmedUserID
		row.CreatedBy = trimmedUserID
	}
}
