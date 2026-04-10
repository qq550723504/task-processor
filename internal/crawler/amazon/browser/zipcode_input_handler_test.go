package browser

import "testing"

func TestInferCountryFromTargetURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{url: "https://www.amazon.com/dp/B001234567", want: "United States"},
		{url: "https://www.amazon.co.uk/dp/B001234567", want: "United Kingdom"},
		{url: "https://www.amazon.ca/dp/B001234567", want: "Canada"},
		{url: "https://www.amazon.co.jp/dp/B001234567", want: "Japan"},
		{url: "https://example.com/item", want: ""},
	}

	for _, tc := range cases {
		if got := inferCountryFromTargetURL(tc.url); got != tc.want {
			t.Fatalf("inferCountryFromTargetURL(%q)=%q want %q", tc.url, got, tc.want)
		}
	}
}

func TestInferCountryFromZipcode(t *testing.T) {
	cases := []struct {
		zipcode string
		want    string
	}{
		{zipcode: "10001", want: "United States"},
		{zipcode: "M5V2T6", want: "Canada"},
		{zipcode: "100-0001", want: "Japan"},
		{zipcode: "SW1A 1AA", want: "United Kingdom"},
		{zipcode: "UNKNOWN", want: ""},
	}

	for _, tc := range cases {
		if got := inferCountryFromZipcode(tc.zipcode); got != tc.want {
			t.Fatalf("inferCountryFromZipcode(%q)=%q want %q", tc.zipcode, got, tc.want)
		}
	}
}

func TestInferDeliveryCountryPrefersTargetURL(t *testing.T) {
	got := inferDeliveryCountry("https://www.amazon.co.uk/dp/B001234567", "10001")
	if got != "United Kingdom" {
		t.Fatalf("inferDeliveryCountry returned %q", got)
	}
}

func TestBuildCountrySelectionQueries(t *testing.T) {
	got := buildCountrySelectionQueries("United States")
	if got != nil {
		t.Fatalf("buildCountrySelectionQueries returned %v", got)
	}
}

func TestShouldRefreshAfterPriceVisibleTriggerFailure(t *testing.T) {
	if !shouldRefreshAfterPriceVisibleTriggerFailure(true, false) {
		t.Fatal("价格已可见且邮编界面未就绪时，应触发刷新")
	}

	if shouldRefreshAfterPriceVisibleTriggerFailure(true, true) {
		t.Fatal("邮编界面已就绪时，不应触发刷新")
	}

	if shouldRefreshAfterPriceVisibleTriggerFailure(false, false) {
		t.Fatal("页面没有价格时，不应因为该规则触发刷新")
	}
}

func TestIsZipcodeSuccessConfirmationText(t *testing.T) {
	if !isZipcodeSuccessConfirmationText("You're now shopping for delivery to: 10001 We will use your selected location to show all products available for the United States. Continue") {
		t.Fatal("应识别 Amazon 邮编更新成功确认态")
	}

	if isZipcodeSuccessConfirmationText("Choose your location or enter a US zip code Apply") {
		t.Fatal("普通邮编输入弹层不应被识别为成功确认态")
	}
}

func TestShouldRefreshAfterSuccessfulZipcodeConfirmation(t *testing.T) {
	if !shouldRefreshAfterSuccessfulZipcodeConfirmation("Japan", "Japan", true) {
		t.Fatal("成功确认后地址未变化时，应触发刷新")
	}

	if !shouldRefreshAfterSuccessfulZipcodeConfirmation("Japan", "", true) {
		t.Fatal("成功确认后地址为空时，应触发刷新")
	}

	if shouldRefreshAfterSuccessfulZipcodeConfirmation("Japan", "New York 10001", true) {
		t.Fatal("成功确认后地址已变化时，不应刷新")
	}

	if shouldRefreshAfterSuccessfulZipcodeConfirmation("Japan", "Japan", false) {
		t.Fatal("没有成功确认弹层时，不应按该规则刷新")
	}
}

func TestZipcodeAddressSyncWaitAfterConfirmIsShorter(t *testing.T) {
	if zipcodeAddressSyncWaitAfterConfirm >= zipcodeAddressSyncWaitDefault {
		t.Fatal("成功确认后的地址同步等待应短于默认等待")
	}
}

func TestBuildCountrySelectionQueriesForUnitedKingdom(t *testing.T) {
	got := buildCountrySelectionQueries("United Kingdom")
	if got != nil {
		t.Fatal("United Kingdom 不应再尝试通过 GLUXCountryList 切换国家")
	}
}
