package money

import "testing"

func TestParseAndFormat(t *testing.T) {
	cases := []struct {
		text     string
		currency string
		minor    int64
		format   string
		invalid  bool
	}{
		{"79", "GBP", 7900, "79.00", false},
		{"1299.99", "GBP", 129999, "1299.99", false},
		{"0.5", "EUR", 50, "0.50", false},
		{"12800", "JPY", 12800, "12800", false},
		{"12.345", "BHD", 12345, "12.345", false},
		{"0.05", "GBP", 5, "0.05", false},
		{"12.5", "JPY", 0, "", true},
		{"1.999", "GBP", 0, "", true},
		{"-5", "GBP", 0, "", true},
		{"1,299.99", "GBP", 0, "", true},
		{"1e3", "GBP", 0, "", true},
		{"", "GBP", 0, "", true},
		{"5.", "GBP", 0, "", true},
		{"99999999999999999999", "GBP", 0, "", true},
	}
	for _, testCase := range cases {
		minor, err := ParseDecimal(testCase.text, testCase.currency)
		if testCase.invalid {
			if err == nil {
				t.Errorf("%q %s accepted as %d", testCase.text, testCase.currency, minor)
			}
			continue
		}
		if err != nil || minor != testCase.minor {
			t.Errorf("%q %s = %d, %v; want %d", testCase.text, testCase.currency, minor, err, testCase.minor)
		}
		if formatted := FormatDecimal(minor, testCase.currency); formatted != testCase.format {
			t.Errorf("format %d %s = %q; want %q", minor, testCase.currency, formatted, testCase.format)
		}
	}
}

func TestNormaliseCurrency(t *testing.T) {
	if code, err := NormaliseCurrency(" gbp "); err != nil || code != "GBP" {
		t.Fatal(code, err)
	}
	for _, bad := range []string{"", "GB", "GBPX", "G1P"} {
		if _, err := NormaliseCurrency(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
