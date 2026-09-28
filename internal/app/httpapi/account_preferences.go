package httpapi

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strings"
	"task-processor/internal/authidentity"
	store "task-processor/internal/integration/persistence/accountprofile"
	"time"
	"unicode"
	"unicode/utf8"
)

func (m accountProfileModule) preferences(c *gin.Context) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || !authidentity.IsBoundedIdentifier(identity.UserID) {
		writeAccountProfileError(c, 401, "AUTHENTICATION_REQUIRED")
		return
	}
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		writeAccountProfileError(c, 400, "INVALID_REQUEST")
		return
	}
	var result store.UserPreferences
	var err error
	if c.Request.Method == http.MethodGet {
		if !accountProfileReadRequest(c.Request) {
			writeAccountProfileError(c, 400, "INVALID_REQUEST")
			return
		}
		result, err = m.repository.ReadPreferences(c.Request.Context(), identity.UserID)
	} else {
		var input struct {
			Country  string `json:"country"`
			Province string `json:"province"`
			City     string `json:"city"`
		}
		if c.ContentType() != "application/json" || c.Request.Body == nil || c.Request.ContentLength > 16384 {
			writeAccountProfileError(c, 400, "INVALID_REQUEST")
			return
		}
		raw, readErr := io.ReadAll(io.LimitReader(c.Request.Body, 16385))
		if !validRegionJSON(raw) {
			writeAccountProfileError(c, 400, "INVALID_REQUEST")
			return
		}
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.DisallowUnknownFields()
		var extra any
		if readErr != nil || len(raw) > 16384 || decoder.Decode(&input) != nil || !errors.Is(decoder.Decode(&extra), io.EOF) {
			writeAccountProfileError(c, 400, "INVALID_REQUEST")
			return
		}
		for _, value := range []string{input.Country, input.Province, input.City} {
			if len(value) > 128 || !utf8.ValidString(value) {
				writeAccountProfileError(c, 400, "INVALID_REQUEST")
				return
			}
			for _, char := range value {
				if unicode.IsControl(char) {
					writeAccountProfileError(c, 400, "INVALID_REQUEST")
					return
				}
			}
		}
		result, err = m.repository.SavePreferences(c.Request.Context(), store.UserPreferences{UserID: identity.UserID, Country: input.Country, Province: input.Province, City: input.City})
	}
	if err != nil {
		writeAccountProfileError(c, 503, "DEPENDENCY_UNAVAILABLE")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(200, gin.H{"schemaVersion": "account-preferences-v1", "userId": identity.UserID, "country": result.Country, "province": result.Province, "city": result.City, "updatedAt": result.UpdatedAt, "readAt": time.Now().UTC(), "source": "account_profile"})
}
func validRegionJSON(raw []byte) bool {
	if !utf8.Valid(raw) {
		return false
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err = decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] || (key != "country" && key != "province" && key != "city") {
			return false
		}
		seen[key] = true
		token, err = decoder.Token()
		if _, ok = token.(string); err != nil || !ok {
			return false
		}
	}
	token, err = decoder.Token()
	if err != nil || token != json.Delim('}') || len(seen) != 3 {
		return false
	}
	_, err = decoder.Token()
	return errors.Is(err, io.EOF)
}
