package browser

import (
	"sync"

	"task-processor/internal/core/config"
)

type riskPolicy struct {
	errorDetector                   *ErrorDetector
	captchaRecreateThreshold        int
	authenticationRecreateThreshold int
	browserCrashRecreateThreshold   int
	timeoutRecreateThreshold        int
	networkRecreateThreshold        int
	serverErrorRecreateThreshold    int
}

type instanceRiskState struct {
	mu                sync.Mutex
	consecutiveByType map[string]int
}

func newRiskPolicy(cfg *config.Config, detector *ErrorDetector) *riskPolicy {
	if detector == nil {
		detector = NewErrorDetector()
	}
	rc := config.AmazonRiskControlConfig{}
	if cfg != nil {
		rc = cfg.Amazon.RiskControl
	}
	return &riskPolicy{
		errorDetector:                   detector,
		captchaRecreateThreshold:        positiveOrDefault(rc.CaptchaRecreateThreshold, 1),
		authenticationRecreateThreshold: positiveOrDefault(rc.AuthenticationRecreateThreshold, 1),
		browserCrashRecreateThreshold:   positiveOrDefault(rc.BrowserCrashRecreateThreshold, 1),
		timeoutRecreateThreshold:        positiveOrDefault(rc.TimeoutRecreateThreshold, 3),
		networkRecreateThreshold:        positiveOrDefault(rc.NetworkRecreateThreshold, 2),
		serverErrorRecreateThreshold:    positiveOrDefault(rc.ServerErrorRecreateThreshold, 3),
	}
}

func (p *riskPolicy) OnSuccess(instance *BrowserInstance) {
	if p == nil || instance == nil {
		return
	}
	instance.riskState.mu.Lock()
	defer instance.riskState.mu.Unlock()
	instance.riskState.consecutiveByType = make(map[string]int)
}

func (p *riskPolicy) OnFailure(instance *BrowserInstance, err error) bool {
	if p == nil || instance == nil || err == nil {
		return false
	}

	errorType := p.errorDetector.GetErrorType(err)
	instance.riskState.mu.Lock()
	defer instance.riskState.mu.Unlock()
	if instance.riskState.consecutiveByType == nil {
		instance.riskState.consecutiveByType = make(map[string]int)
	}

	for key := range instance.riskState.consecutiveByType {
		if key != errorType {
			delete(instance.riskState.consecutiveByType, key)
		}
	}
	instance.riskState.consecutiveByType[errorType]++
	count := instance.riskState.consecutiveByType[errorType]

	switch errorType {
	case "captcha":
		return count >= p.captchaRecreateThreshold
	case "authentication":
		return count >= p.authenticationRecreateThreshold
	case "browser_crash":
		return count >= p.browserCrashRecreateThreshold
	case "timeout":
		return count >= p.timeoutRecreateThreshold
	case "network":
		return count >= p.networkRecreateThreshold
	case "server_error":
		return count >= p.serverErrorRecreateThreshold
	default:
		return p.errorDetector.IsBlockedOrSeriousError(err)
	}
}

func (p *riskPolicy) ShouldSyncRecreateAfterFailure(instance *BrowserInstance, err error) bool {
	if p == nil || instance == nil || err == nil {
		return false
	}
	errorType := p.errorDetector.GetErrorType(err)
	switch errorType {
	case "captcha", "authentication", "browser_crash":
		return p.OnFailure(instance, err)
	default:
		return false
	}
}

func positiveOrDefault(value, defaultValue int) int {
	if value > 0 {
		return value
	}
	return defaultValue
}
