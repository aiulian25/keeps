package extract

import (
	"errors"
	"strconv"
	"strings"
)

const (
	defaultCurrencyExponent = 2
	currencyCodeLength      = 3
	thousandsGroupLength    = 3
	maxLeadingGroupLength   = 3
	decimalBase             = 10
	minorUnitsBitSize       = 64
	amountDigits            = "0123456789"
	amountCharacters        = amountDigits + ".,"
	dotSeparator            = "."
	commaSeparator          = ","
)

var (
	errInvalidCurrency = errors.New("currency is not an ISO 4217 code")
	errInvalidAmount   = errors.New("amount is not a plain decimal number")
	errAmbiguousAmount = errors.New("amount separators are ambiguous")
	errTooPrecise      = errors.New("amount has more decimals than the currency allows")
)

var zeroExponentCurrencies = map[string]bool{
	"BIF": true, "CLP": true, "DJF": true, "GNF": true, "ISK": true, "JPY": true,
	"KMF": true, "KRW": true, "PYG": true, "RWF": true, "UGX": true, "UYI": true,
	"VND": true, "VUV": true, "XAF": true, "XOF": true, "XPF": true,
}

var threeExponentCurrencies = map[string]bool{
	"BHD": true, "IQD": true, "JOD": true, "KWD": true, "LYD": true, "OMR": true, "TND": true,
}

// Currencies whose shops commonly write "1.299,99"; only for these is a comma
// accepted as the decimal separator.
var commaDecimalCurrencies = map[string]bool{
	"EUR": true, "DKK": true, "NOK": true, "SEK": true, "PLN": true, "CZK": true,
	"HUF": true, "RON": true, "BGN": true, "BRL": true, "ARS": true, "TRY": true,
}

func currencyExponent(code string) int {
	if zeroExponentCurrencies[code] {
		return 0
	}
	if threeExponentCurrencies[code] {
		return 3
	}
	return defaultCurrencyExponent
}

func isCurrencyCode(code string) bool {
	if len(code) != currencyCodeLength {
		return false
	}
	for _, letter := range code {
		if letter < 'A' || letter > 'Z' {
			return false
		}
	}
	return true
}

// parseMoney converts a decimal string to exact minor units without floats.
func parseMoney(amount, currency string) (Money, error) {
	code := strings.ToUpper(strings.TrimSpace(currency))
	if !isCurrencyCode(code) {
		return Money{}, errInvalidCurrency
	}
	integerPart, fractionPart, err := splitAmount(strings.TrimSpace(amount), code)
	if err != nil {
		return Money{}, err
	}
	exponent := currencyExponent(code)
	fractionPart = strings.TrimRight(fractionPart, "0")
	if len(fractionPart) > exponent {
		return Money{}, errTooPrecise
	}
	digits := integerPart + fractionPart + strings.Repeat("0", exponent-len(fractionPart))
	minorUnits, err := strconv.ParseInt(digits, decimalBase, minorUnitsBitSize)
	if err != nil {
		return Money{}, errInvalidAmount
	}
	return Money{MinorUnits: minorUnits, Currency: code}, nil
}

func priceOrNil(amount, currency string) *Money {
	money, err := parseMoney(amount, currency)
	if err != nil {
		return nil
	}
	return &money
}

func pricedOffers(amount, currency string) []Offer {
	price := priceOrNil(amount, currency)
	if price == nil {
		return nil
	}
	return []Offer{{Price: price}}
}

func splitAmount(amount, code string) (string, string, error) {
	if amount == "" || strings.Trim(amount, amountCharacters) != "" || strings.IndexAny(amount, amountDigits) < 0 {
		return "", "", errInvalidAmount
	}
	lastDot := strings.LastIndex(amount, dotSeparator)
	lastComma := strings.LastIndex(amount, commaSeparator)
	hasDot := lastDot >= 0
	hasComma := lastComma >= 0
	if !hasDot && !hasComma {
		return amount, "", nil
	}
	if !hasComma {
		return splitSingleSeparator(amount, dotSeparator, code)
	}
	if !hasDot {
		return splitSingleSeparator(amount, commaSeparator, code)
	}
	if lastDot > lastComma {
		return splitGroupedDecimal(amount, commaSeparator, dotSeparator)
	}
	if !commaDecimalCurrencies[code] {
		return "", "", errAmbiguousAmount
	}
	return splitGroupedDecimal(amount, dotSeparator, commaSeparator)
}

// splitSingleSeparator handles amounts using only one kind of separator. One
// occurrence followed by exactly three digits could be either a thousands
// group or a decimal fraction, so it is only read as a fraction for
// three-decimal currencies written with a dot.
func splitSingleSeparator(amount, separator, code string) (string, string, error) {
	isDecimalSeparator := separator == dotSeparator || commaDecimalCurrencies[code]
	if strings.Count(amount, separator) > 1 || !isDecimalSeparator {
		integerPart, err := ungroup(amount, separator)
		return integerPart, "", err
	}
	integerPart, fractionPart, _ := strings.Cut(amount, separator)
	looksLikeThousands := len(fractionPart) == thousandsGroupLength && currencyExponent(code) != thousandsGroupLength
	if looksLikeThousands {
		return "", "", errAmbiguousAmount
	}
	if integerPart == "" || fractionPart == "" {
		return "", "", errInvalidAmount
	}
	return integerPart, fractionPart, nil
}

func splitGroupedDecimal(amount, groupSeparator, decimalSeparator string) (string, string, error) {
	decimalIndex := strings.LastIndex(amount, decimalSeparator)
	integerText := amount[:decimalIndex]
	fractionPart := amount[decimalIndex+len(decimalSeparator):]
	if fractionPart == "" || strings.Contains(integerText, decimalSeparator) {
		return "", "", errAmbiguousAmount
	}
	integerPart, err := ungroup(integerText, groupSeparator)
	if err != nil {
		return "", "", err
	}
	return integerPart, fractionPart, nil
}

func ungroup(text, separator string) (string, error) {
	groups := strings.Split(text, separator)
	leading := groups[0]
	if leading == "" || len(leading) > maxLeadingGroupLength {
		return "", errAmbiguousAmount
	}
	for _, group := range groups[1:] {
		if len(group) != thousandsGroupLength {
			return "", errAmbiguousAmount
		}
	}
	return strings.Join(groups, ""), nil
}
