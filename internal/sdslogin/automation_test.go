package sdslogin

import (
	"fmt"
	"strings"
	"testing"
)

type fakeLoginPage struct {
	locators map[string]*fakeLoginLocator
}

func (p fakeLoginPage) Locator(selector string) loginLocator {
	if locator, ok := p.locators[selector]; ok {
		return locator
	}
	return &fakeLoginLocator{}
}

type fakeLoginLocator struct {
	count            int
	value            string
	typeValue        string
	fillValue        string
	evaluateValue    string
	clickErr         error
	fillErr          error
	typeErr          error
	inputValueErr    error
	evaluateErr      error
	supportsInputAPI bool
}

func (l *fakeLoginLocator) Count() (int, error) {
	if l.count == 0 {
		return 0, nil
	}
	return l.count, nil
}

func (l *fakeLoginLocator) Click() error { return l.clickErr }

func (l *fakeLoginLocator) Press(_ string) error { return nil }

func (l *fakeLoginLocator) Fill(_ string) error {
	if l.fillErr != nil {
		return l.fillErr
	}
	l.value = l.fillValue
	return nil
}

func (l *fakeLoginLocator) Type(_ string) error {
	if l.typeErr != nil {
		return l.typeErr
	}
	l.value = l.typeValue
	return nil
}

func (l *fakeLoginLocator) InputValue() (string, error) {
	if l.inputValueErr != nil {
		return "", l.inputValueErr
	}
	if !l.supportsInputAPI {
		return "", fmt.Errorf("input value unavailable")
	}
	return l.value, nil
}

func (l *fakeLoginLocator) Evaluate(_ string, _ any) (any, error) {
	if l.evaluateErr != nil {
		return nil, l.evaluateErr
	}
	l.value = l.evaluateValue
	return l.value, nil
}

func TestPrefillLoginFormReturnsErrorWhenFieldValueDoesNotStick(t *testing.T) {
	page := fakeLoginPage{
		locators: map[string]*fakeLoginLocator{
			`input[placeholder*="商户"]`: {
				count:            1,
				typeValue:        "",
				fillValue:        "",
				evaluateValue:    "",
				supportsInputAPI: true,
			},
			`input[placeholder*="手机"]`: {
				count:            1,
				typeValue:        "",
				fillValue:        "",
				evaluateValue:    "",
				supportsInputAPI: true,
			},
			`input[type="password"]`: {
				count:            1,
				typeValue:        "secret",
				fillValue:        "secret",
				evaluateValue:    "secret",
				supportsInputAPI: true,
			},
		},
	}

	err := prefillLoginFormWithPage(page, configuredAccount{
		MerchantName: "pod",
		Username:     "zone",
		Password:     "secret",
	})
	if err == nil {
		t.Fatal("expected prefill to fail when merchant_name cannot be written")
	}
	if !strings.Contains(err.Error(), "merchant_name") {
		t.Fatalf("expected merchant_name error, got %v", err)
	}
}

func TestPrefillLoginFormFallsBackToScriptedValueSet(t *testing.T) {
	page := fakeLoginPage{
		locators: map[string]*fakeLoginLocator{
			`#merchant_name`: {
				count:            1,
				typeValue:        "",
				fillValue:        "",
				evaluateValue:    "pod",
				supportsInputAPI: true,
			},
			`#username`: {
				count:            1,
				typeValue:        "",
				fillValue:        "",
				evaluateValue:    "zone",
				supportsInputAPI: true,
			},
			`#password`: {
				count:            1,
				typeValue:        "",
				fillValue:        "",
				evaluateValue:    "secret",
				supportsInputAPI: true,
			},
		},
	}

	err := prefillLoginFormWithPage(page, configuredAccount{
		MerchantName: "pod",
		Username:     "zone",
		Password:     "secret",
	})
	if err != nil {
		t.Fatalf("expected scripted fallback to succeed, got %v", err)
	}
}

func TestHasUsableLoginStateDoesNotRequireCookiesAfterLeavingLoginPage(t *testing.T) {
	state := &pageLoginState{
		Token:      "token",
		MerchantID: 36811,
		UserID:     30098709,
		Href:       "https://www.sdsdiy.com/admin/material",
	}

	if !hasUsableLoginState(state) {
		t.Fatal("expected login state with access token to be usable after leaving login page")
	}

	state.Href = "https://www.sdsdiy.com/user/login?redirect=%2Fadmin%2Fmaterial"
	if hasUsableLoginState(state) {
		t.Fatal("expected login page state without cookies to remain unusable")
	}
}
