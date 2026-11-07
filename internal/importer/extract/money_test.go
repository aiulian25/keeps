package extract

import "testing"

func TestParseMoney(t *testing.T) {
	accepted := []struct {
		amount, currency string
		minorUnits       int64
		wantCurrency     string
	}{
		{"1299", "GBP", 129900, "GBP"},
		{"1299.99", "GBP", 129999, "GBP"},
		{"1,299.99", "USD", 129999, "USD"},
		{"1,299", "USD", 129900, "USD"},
		{"12.5", "GBP", 1250, "GBP"},
		{"0", "GBP", 0, "GBP"},
		{" 19.99 ", " gbp ", 1999, "GBP"},
		{"12800", "JPY", 12800, "JPY"},
		{"12800.00", "JPY", 12800, "JPY"},
		{"1,500", "KRW", 1500, "KRW"},
		{"12.345", "BHD", 12345, "BHD"},
		{"1.5", "KWD", 1500, "KWD"},
		{"1.299,99", "EUR", 129999, "EUR"},
		{"12,50", "EUR", 1250, "EUR"},
		{"1.234.567,89", "EUR", 123456789, "EUR"},
		{"1,234,567.89", "EUR", 123456789, "EUR"},
	}
	for _, test := range accepted {
		money, err := parseMoney(test.amount, test.currency)
		if err != nil {
			t.Errorf("parseMoney(%q, %q) error %v", test.amount, test.currency, err)
			continue
		}
		if money.MinorUnits != test.minorUnits || money.Currency != test.wantCurrency {
			t.Errorf("parseMoney(%q, %q) = %+v, want %d %s", test.amount, test.currency, money, test.minorUnits, test.wantCurrency)
		}
	}

	rejected := []struct{ amount, currency string }{
		{"", "GBP"},
		{"12.99", ""},
		{"12.99", "POUNDS"},
		{"12.99", "G1P"},
		{"£12.99", "GBP"},
		{"-5", "GBP"},
		{"1e3", "GBP"},
		{"12.999", "GBP"},
		{"12.5", "JPY"},
		{"1.000", "GBP"},
		{"1,299", "EUR"},
		{"1.299,99", "GBP"},
		{"12,50", "GBP"},
		{"1,2,3.45", "USD"},
		{"1.2,3.45", "USD"},
		{".99", "GBP"},
		{"99.", "GBP"},
		{"..", "GBP"},
		{"1 299", "GBP"},
		{"99999999999999999999", "GBP"},
	}
	for _, test := range rejected {
		if money, err := parseMoney(test.amount, test.currency); err == nil {
			t.Errorf("parseMoney(%q, %q) = %+v, want error", test.amount, test.currency, money)
		}
	}
}

func TestCurrencyExponent(t *testing.T) {
	exponents := map[string]int{"JPY": 0, "KRW": 0, "BHD": 3, "KWD": 3, "OMR": 3, "JOD": 3, "TND": 3, "GBP": 2, "EUR": 2, "USD": 2}
	for code, want := range exponents {
		if got := currencyExponent(code); got != want {
			t.Errorf("currencyExponent(%s) = %d, want %d", code, got, want)
		}
	}
}
