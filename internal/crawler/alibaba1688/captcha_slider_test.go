package alibaba1688

import "testing"

func TestOfferURLWithCaptchaTitleIsNotReadyAfterCaptcha(t *testing.T) {
	if isProductPageReadyAfterCaptcha("Captcha Interception", false) {
		t.Fatal("offer URL with captcha title and no page data should not be treated as ready")
	}
}

func TestOfferURLWithProductDataIsReadyAfterCaptcha(t *testing.T) {
	if !isProductPageReadyAfterCaptcha("Captcha Interception", true) {
		t.Fatal("offer URL with page data should be treated as ready")
	}
}

func TestOfferURLWithProductTitleIsReadyAfterCaptcha(t *testing.T) {
	if !isProductPageReadyAfterCaptcha("Wholesale Product", false) {
		t.Fatal("offer URL with non-captcha title should be treated as ready")
	}
}
