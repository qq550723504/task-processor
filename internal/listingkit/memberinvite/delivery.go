package memberinvite

import (
	"strings"
	"unicode/utf8"
)

// DeliveryMetadata is the safe, server-derived representation of invitation contacts.
type DeliveryMetadata struct {
	Mode    string
	Email   string
	Phone   string
	Contact string
}

// DeliveryMetadataFor masks invitation contacts and identifies the selected delivery mode.
func DeliveryMetadataFor(email, phone string) DeliveryMetadata {
	maskedEmail := maskEmail(email)
	maskedPhone := maskPhone(phone)
	metadata := DeliveryMetadata{
		Mode:    "email",
		Email:   maskedEmail,
		Phone:   maskedPhone,
		Contact: maskedEmail,
	}
	if maskedPhone != "" {
		metadata.Mode = "email_phone"
		metadata.Contact += "," + maskedPhone
	}
	return metadata
}

func maskEmail(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return "***"
	}
	local := email[:at]
	if len(local) == 1 {
		return "*" + email[at:]
	}
	firstRune, _ := utf8.DecodeRuneInString(local)
	return string(firstRune) + "***" + email[at:]
}

func maskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	if len(phone) <= 5 {
		return "***"
	}
	return phone[:3] + "***" + phone[len(phone)-4:]
}
