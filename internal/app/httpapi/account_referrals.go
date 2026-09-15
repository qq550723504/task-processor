package httpapi

import (
	"github.com/gin-gonic/gin"
	"task-processor/internal/referral"
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
	writeReferralJSON(c, gin.H{"code": out.Code, "codeAvailability": availability, "count": out.Count, "generatedAt": time.Now().UTC(), "earnings": gin.H{"availability": "unavailable", "amount": nil}})
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
