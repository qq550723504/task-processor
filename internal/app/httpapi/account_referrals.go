package httpapi

import (
	"github.com/gin-gonic/gin"
	"strconv"
	"task-processor/internal/authidentity"
	"task-processor/internal/referral"
	economics "task-processor/internal/referraleconomics"
	"time"
)

func referralSelfInput(c *gin.Context) bool {
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery || c.Request.ContentLength != 0 || len(c.Request.TransferEncoding) != 0 || c.GetHeader("X-Referral-Service-Credential") != "" {
		writeReferralError(c, referral.ErrInvalid)
		return false
	}
	return true
}
func (m referralHTTPModule) readSelf(c *gin.Context) {
	if !referralSelfInput(c) {
		return
	}
	out, err := m.commands.ReadSelf(c.Request.Context())
	if err != nil {
		writeReferralError(c, err)
		return
	}
	availability := "available"
	if out.Code == "" {
		availability = "not_created"
	}
	earnings := gin.H{"availability": "unavailable", "amount": nil}
	if m.economics != nil {
		if identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context()); ok {
			if value, err := m.economics.ReadEarnings(c.Request.Context(), identity.UserID, economics.CurrencyCNY); err == nil {
				earnings = gin.H{"availability": "available", "currency": value.Currency, "pendingMinor": strconv.FormatInt(value.PendingMinor, 10), "availableMinor": strconv.FormatInt(value.AvailableMinor, 10), "reservedMinor": strconv.FormatInt(value.ReservedMinor, 10), "adjustmentMinor": strconv.FormatInt(value.AdjustmentMinor, 10), "version": strconv.FormatInt(value.Version, 10), "updatedAt": nullableTime(value.UpdatedAt)}
			}
		}
	}
	writeReferralJSON(c, gin.H{"code": out.Code, "codeAvailability": availability, "count": out.Count, "generatedAt": time.Now().UTC(), "earnings": earnings})
}
func (m referralHTTPModule) createSelfCode(c *gin.Context) {
	if !referralSelfInput(c) {
		return
	}
	code, err := m.commands.CreateSelfCode(c.Request.Context())
	if err != nil {
		writeReferralError(c, err)
		return
	}
	writeReferralJSON(c, gin.H{"code": code})
}
func (m referralHTTPModule) complete(c *gin.Context) {
	if !referralSelfInput(c) {
		return
	}
	out, err := m.commands.Complete(c.Request.Context())
	if err != nil {
		writeReferralError(c, err)
		return
	}
	writeReferralJSON(c, gin.H{"status": "complete", "intentID": out.IntentID, "boundAt": out.BoundAt.UTC()})
}
