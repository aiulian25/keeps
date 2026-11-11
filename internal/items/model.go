// Package items holds belongings and the purchase evidence attached to them.
package items

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/money"
)

// Conditions lists the allowed item conditions; "" means not recorded.
var Conditions = map[string]bool{"": true, "new": true, "like_new": true, "good": true, "fair": true, "poor": true, "needs_repair": true}

const (
	PrecisionDate     = "date"
	PrecisionDateTime = "datetime"
	dateLayout        = "2006-01-02"
	localTimeLayout   = "2006-01-02T15:04"
	defaultQuantity   = 1
	maximumQuantity   = 100000
	maximumGroupNames = 50
)

var dateTimeLayouts = []string{localTimeLayout, "2006-01-02T15:04:05", time.RFC3339}

type Money struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type PurchaseDate struct {
	Value     string `json:"value"`
	Precision string `json:"precision"`
}

// Input is every user-editable field of an item. PUT replaces all of them.
type Input struct {
	Name           string        `json:"name"`
	Description    string        `json:"description"`
	Quantity       int           `json:"quantity"`
	Brand          string        `json:"brand"`
	Model          string        `json:"model"`
	SerialNumber   string        `json:"serialNumber"`
	Condition      string        `json:"condition"`
	WarrantyUntil  string        `json:"warrantyUntil"`
	Measurements   *Measurements `json:"measurements"`
	Notes          string        `json:"notes"`
	Seller         string        `json:"seller"`
	OrderReference string        `json:"orderReference"`
	PricePaid      *Money        `json:"pricePaid"`
	PurchaseDate   *PurchaseDate `json:"purchaseDate"`
	Room           string        `json:"room"`
	Category       string        `json:"category"`
	Tags           []string      `json:"tags"`
	Collections    []string      `json:"collections"`
	PurchaseID     int64         `json:"purchaseId"`
	AttachmentIDs  []int64       `json:"attachmentIds"`
}

// normalised is Input after validation, with money in minor units.
type normalised struct {
	Input
	paidMinor    *int64
	paidCurrency string
	purchasedOn  string
	precision    string
}

func (n normalised) hasPurchaseDetails() bool {
	return n.Seller != "" || n.OrderReference != "" || n.purchasedOn != ""
}

func clean(value string) string {
	return strings.TrimSpace(value)
}

func tooLong(value string, maximum int) bool {
	return utf8.RuneCountInString(value) > maximum
}

// validate returns field-level errors so the form can show them inline while
// keeping everything the user typed.
func validate(input Input, householdCurrency string) (normalised, error) {
	result := normalised{Input: input}
	result.Name = clean(input.Name)
	result.Description = clean(input.Description)
	result.Brand = clean(input.Brand)
	result.Model = clean(input.Model)
	result.SerialNumber = clean(input.SerialNumber)
	result.Condition = clean(input.Condition)
	result.WarrantyUntil = clean(input.WarrantyUntil)
	result.Notes = strings.TrimRight(input.Notes, " \n\t")
	result.Seller = clean(input.Seller)
	result.OrderReference = clean(input.OrderReference)
	result.Room = clean(input.Room)
	result.Category = clean(input.Category)
	result.Tags = uniqueNames(input.Tags)
	result.Collections = uniqueNames(input.Collections)

	fields := map[string]string{}
	if result.Name == "" {
		fields["name"] = "Enter a name."
	}
	if tooLong(result.Name, config.ItemNameMaximumLength) {
		fields["name"] = "Use up to 200 characters."
	}
	for field, value := range map[string]string{"description": result.Description, "notes": result.Notes} {
		if tooLong(value, config.ItemTextMaximumLength) {
			fields[field] = "This is too long."
		}
	}
	shortFields := map[string]string{"brand": result.Brand, "model": result.Model, "serialNumber": result.SerialNumber, "seller": result.Seller, "orderReference": result.OrderReference}
	for field, value := range shortFields {
		if tooLong(value, config.ItemShortTextMaximum) {
			fields[field] = "Use up to 200 characters."
		}
	}
	if result.Quantity == 0 {
		result.Quantity = defaultQuantity
	}
	if result.Quantity < defaultQuantity || result.Quantity > maximumQuantity {
		fields["quantity"] = "Enter a whole number of 1 or more."
	}
	if !Conditions[result.Condition] {
		fields["condition"] = "Choose a condition from the list."
	}
	if result.WarrantyUntil != "" {
		if _, err := time.Parse(dateLayout, result.WarrantyUntil); err != nil {
			fields["warrantyUntil"] = "Enter a real date."
		}
	}
	result.Measurements = normaliseMeasurements(input.Measurements, fields)
	if len(result.Tags) > maximumGroupNames || len(result.Collections) > maximumGroupNames {
		fields["tags"] = "Too many tags or collections."
	}

	if input.PricePaid != nil && clean(input.PricePaid.Amount) != "" {
		currency := input.PricePaid.Currency
		if clean(currency) == "" {
			currency = householdCurrency
		}
		normalisedCurrency, err := money.NormaliseCurrency(currency)
		if err != nil {
			fields["pricePaid"] = "Use a three-letter currency code such as GBP."
		}
		amount, plain := plainAmount(input.PricePaid.Amount)
		if !plain && fields["pricePaid"] == "" {
			fields["pricePaid"] = "Use a dot for decimals, like 79.99. Commas may only separate thousands in an amount with a dot, like 1,234.50."
		}
		minor, err := money.ParseDecimal(amount, normalisedCurrency)
		if err != nil && fields["pricePaid"] == "" {
			fields["pricePaid"] = "Enter an amount like 79 or 79.99."
		}
		if err == nil {
			result.paidMinor = &minor
			result.paidCurrency = normalisedCurrency
		}
	}

	if input.PurchaseDate != nil && clean(input.PurchaseDate.Value) != "" {
		value, precision, ok := parsePurchaseDate(*input.PurchaseDate)
		if !ok {
			fields["purchaseDate"] = "Enter a real date."
		}
		result.purchasedOn, result.precision = value, precision
	}

	if len(fields) > 0 {
		return normalised{}, &httpapi.Error{Status: 400, Code: "invalid_item", Message: "Check the highlighted fields.", Fields: fields}
	}
	return result, nil
}

// groupedAmount is a number with comma thousands separators and a decimal
// dot, the only form in which a comma's meaning is not a guess.
var groupedAmount = regexp.MustCompile(`^\d{1,3}(,\d{3})+\.\d+$`)

// plainAmount removes thousands separators only where they are unambiguous.
// "79,99" or "1,234" could be a decimal comma, so they are refused rather
// than silently becoming 7999 or 1234.
func plainAmount(raw string) (string, bool) {
	amount := clean(raw)
	if !strings.Contains(amount, ",") {
		return amount, true
	}
	if groupedAmount.MatchString(amount) {
		return strings.ReplaceAll(amount, ",", ""), true
	}
	return amount, false
}

// parsePurchaseDate keeps exactly the precision the user gave. A date never
// gains an invented time, and nothing defaults to today.
func parsePurchaseDate(date PurchaseDate) (string, string, bool) {
	value := clean(date.Value)
	if date.Precision == PrecisionDate || len(value) == len(dateLayout) {
		parsed, err := time.Parse(dateLayout, value)
		return parsed.Format(dateLayout), PrecisionDate, err == nil
	}
	for _, layout := range dateTimeLayouts {
		if _, err := time.Parse(layout, value); err == nil {
			return value, PrecisionDateTime, true
		}
	}
	return "", "", false
}

func uniqueNames(names []string) []string {
	seen := map[string]bool{}
	unique := []string{}
	for _, name := range names {
		name = strings.Join(strings.Fields(name), " ")
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, name)
	}
	return unique
}

type Reference struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Attachment struct {
	ID         int64  `json:"id"`
	Kind       string `json:"kind"`
	Filename   string `json:"filename"`
	MediaType  string `json:"mediaType"`
	SizeBytes  int64  `json:"sizeBytes"`
	CreatedAt  string `json:"createdAt"`
	HasPreview bool   `json:"hasPreview"`
	Shared     bool   `json:"shared"`
}

type Listing struct {
	Title       string `json:"title"`
	Price       *Money `json:"price"`
	SourceURL   string `json:"sourceUrl"`
	SourceHost  string `json:"sourceHost"`
	RetrievedAt string `json:"retrievedAt"`
}

// Item is the full detail representation.
type Item struct {
	ID                 int64             `json:"id"`
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	Quantity           int               `json:"quantity"`
	Brand              string            `json:"brand"`
	Model              string            `json:"model"`
	SerialNumber       string            `json:"serialNumber"`
	Condition          string            `json:"condition"`
	WarrantyUntil      string            `json:"warrantyUntil"`
	Measurements       *Measurements     `json:"measurements"`
	Notes              string            `json:"notes"`
	Seller             string            `json:"seller"`
	OrderReference     string            `json:"orderReference"`
	PricePaid          *Money            `json:"pricePaid"`
	PurchaseDate       *PurchaseDate     `json:"purchaseDate"`
	PurchaseID         int64             `json:"purchaseId"`
	SharedWith         []Reference       `json:"sharedWith"`
	Room               *Reference        `json:"room"`
	Category           *Reference        `json:"category"`
	Tags               []Reference       `json:"tags"`
	Collections        []Reference       `json:"collections"`
	PrimaryPhotoID     int64             `json:"primaryPhotoId"`
	Photos             []Attachment      `json:"photos"`
	Documents          []Attachment      `json:"documents"`
	Listing            *Listing          `json:"listing"`
	Parts              []Part            `json:"parts"`
	Units              []Unit            `json:"units"`
	SourceURL          string            `json:"sourceUrl"`
	FieldSources       map[string]string `json:"fieldSources"`
	Revision           int64             `json:"revision"`
	CreatedAt          string            `json:"createdAt"`
	UpdatedAt          string            `json:"updatedAt"`
	DeletedAt          string            `json:"deletedAt,omitempty"`
	PossibleDuplicates []Reference       `json:"possibleDuplicates,omitempty"`
}

// Summary is the compact representation used in lists and cards.
type Summary struct {
	ID             int64         `json:"id"`
	Name           string        `json:"name"`
	PricePaid      *Money        `json:"pricePaid"`
	PurchaseDate   *PurchaseDate `json:"purchaseDate"`
	Room           *Reference    `json:"room"`
	PrimaryPhotoID int64         `json:"primaryPhotoId"`
	HasReceipt     bool          `json:"hasReceipt"`
	CreatedAt      string        `json:"createdAt"`
	DeletedAt      string        `json:"deletedAt,omitempty"`
}

type Page struct {
	Items      []Summary `json:"items"`
	Total      int       `json:"total"`
	NextCursor string    `json:"nextCursor,omitempty"`
}

type Revision struct {
	Source    string         `json:"source"`
	Changes   map[string]any `json:"changes"`
	ChangedAt string         `json:"changedAt"`
}

// ImportOrigin describes a reviewed import being saved. Field values that
// still equal the imported values are recorded with import provenance.
type ImportOrigin struct {
	SourceURL        string
	SourceHost       string
	Title            string
	ListingPrice     *int64
	ListingCurrency  string
	RetrievedAt      string
	ExtractorVersion string
	ImportedValues   map[string]string
}
