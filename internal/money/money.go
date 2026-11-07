// Package money parses and formats exact amounts as integer minor units.
package money

import (
	"errors"
	"math"
	"strings"
)

const (
	currencyCodeLength = 3
	defaultExponent    = 2
	decimalBase        = 10
)

// exponentExceptions lists ISO 4217 currencies whose minor unit is not 1/100.
var exponentExceptions = map[string]int{
	"BIF": 0, "CLP": 0, "DJF": 0, "GNF": 0, "ISK": 0, "JPY": 0, "KMF": 0, "KRW": 0,
	"PYG": 0, "RWF": 0, "UGX": 0, "UYI": 0, "VND": 0, "VUV": 0, "XAF": 0, "XOF": 0, "XPF": 0,
	"BHD": 3, "IQD": 3, "JOD": 3, "KWD": 3, "LYD": 3, "OMR": 3, "TND": 3,
}

var (
	ErrInvalidCurrency = errors.New("currency must be a three-letter ISO 4217 code")
	ErrInvalidAmount   = errors.New("amount must be a non-negative number with at most the currency's decimal places")
)

// Exponent returns how many decimal places the currency uses.
func Exponent(currency string) int {
	exponent, found := exponentExceptions[currency]
	if found {
		return exponent
	}
	return defaultExponent
}

func NormaliseCurrency(currency string) (string, error) {
	upper := strings.ToUpper(strings.TrimSpace(currency))
	if len(upper) != currencyCodeLength {
		return "", ErrInvalidCurrency
	}
	for _, letter := range upper {
		if letter < 'A' || letter > 'Z' {
			return "", ErrInvalidCurrency
		}
	}
	return upper, nil
}

// ParseDecimal converts a plain decimal string such as "1299.99" into minor
// units for the currency. Thousands separators are not accepted here; the UI
// sends plain decimals.
func ParseDecimal(text string, currency string) (int64, error) {
	text = strings.TrimSpace(text)
	whole, fraction, hasFraction := strings.Cut(text, ".")
	exponent := Exponent(currency)

	invalid := whole == "" || len(fraction) > exponent || (hasFraction && fraction == "")
	if invalid {
		return 0, ErrInvalidAmount
	}
	fraction += strings.Repeat("0", exponent-len(fraction))

	var total int64
	for _, digit := range whole + fraction {
		if digit < '0' || digit > '9' {
			return 0, ErrInvalidAmount
		}
		if total > (math.MaxInt64-int64(digit-'0'))/decimalBase {
			return 0, ErrInvalidAmount
		}
		total = total*decimalBase + int64(digit-'0')
	}
	return total, nil
}

// FormatDecimal renders minor units as a plain decimal string ("1299.99").
func FormatDecimal(minorUnits int64, currency string) string {
	exponent := Exponent(currency)
	digits := []byte(itoa(minorUnits))
	if exponent == 0 {
		return string(digits)
	}
	for len(digits) <= exponent {
		digits = append([]byte{'0'}, digits...)
	}
	split := len(digits) - exponent
	return string(digits[:split]) + "." + string(digits[split:])
}

func itoa(value int64) string {
	if value == 0 {
		return "0"
	}
	var buffer [20]byte
	position := len(buffer)
	for value > 0 {
		position--
		buffer[position] = byte('0' + value%decimalBase)
		value /= decimalBase
	}
	return string(buffer[position:])
}
