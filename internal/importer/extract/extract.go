// Package extract turns a fetched public product page into draft candidates.
// A page describes a listing (the current offer), never what the buyer paid
// or when, so no purchase date or paid price is ever produced here.
package extract

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

// Version is recorded as the extractor version in field provenance.
const Version = "extract-1"

const (
	maxDocumentBytes      = 5 << 20
	maxCandidates         = 10
	maxImageURLs          = 5
	maxRawImageURLs       = 50
	maxImageURLBytes      = 2048
	maxOffersPerCandidate = 10
	maxNameRunes          = 300
	maxDescriptionRunes   = 5000
	maxFieldRunes         = 200
	secureScheme          = "https"
)

const (
	SourceJSONLD    = "json-ld"
	SourceMicrodata = "microdata"
	SourceOpenGraph = "opengraph"
	SourceHTML      = "html"
)

const (
	FieldName        = "name"
	FieldDescription = "description"
	FieldBrand       = "brand"
	FieldModel       = "model"
	FieldSKU         = "sku"
	FieldImages      = "images"
	FieldPrice       = "price"
)

var errInvalidPageURL = errors.New("page URL must be absolute")

// Money is an exact amount in the currency's minor units.
type Money struct {
	MinorUnits int64
	Currency   string
}

// Offer is a listing offer; its price is never the price paid.
type Offer struct {
	Price        *Money
	Availability string
	Seller       string
	isAggregate  bool
}

// Candidate is one product found on the page.
type Candidate struct {
	Name, Description, Brand, Model, SKU string
	ImageURLs                            []string
	Offers                               []Offer
}

type Result struct {
	// More than one candidate means variants or several products: the user chooses.
	Candidates []Candidate
	PageTitle  string
	// Sources maps a field name to the earliest source that supplied it.
	Sources map[string]string
	// Partial is true when any candidate lacks a name or an image.
	Partial bool
}

type sourceLayer struct {
	source     string
	candidates []Candidate
}

// Extract reads structured data from document, treating all of it as
// untrusted text. Sources are consulted in order JSON-LD, microdata,
// Open Graph, plain HTML; earlier sources win per field.
func Extract(pageURL *url.URL, document []byte) (Result, error) {
	if pageURL == nil || !pageURL.IsAbs() {
		return Result{}, errInvalidPageURL
	}
	if len(document) > maxDocumentBytes {
		document = document[:maxDocumentBytes]
	}
	root, err := html.Parse(bytes.NewReader(document))
	if err != nil {
		return Result{}, fmt.Errorf("parse document: %w", err)
	}
	metadata := extractPageMetadata(root)
	layers := []sourceLayer{
		{source: SourceJSONLD, candidates: extractJSONLD(root)},
		{source: SourceMicrodata, candidates: extractMicrodata(root)},
		{source: SourceOpenGraph, candidates: []Candidate{metadata.openGraphCandidate()}},
		{source: SourceHTML, candidates: []Candidate{metadata.htmlCandidate()}},
	}
	result := Result{
		PageTitle: cleanText(metadata.documentTitle, maxNameRunes),
		Sources:   map[string]string{},
	}
	for _, layer := range layers {
		result.mergeLayer(layer.source, normalizeCandidates(layer.candidates, pageURL))
	}
	result.Partial = isPartial(result.Candidates)
	return result, nil
}

func (result *Result) mergeLayer(source string, candidates []Candidate) {
	if len(candidates) == 0 {
		return
	}
	if len(result.Candidates) == 0 {
		result.Candidates = candidates
		for _, candidate := range candidates {
			empty := Candidate{}
			result.recordSources(source, fillMissingFields(&empty, candidate))
		}
		return
	}
	fallback := candidates[0]
	for index := range result.Candidates {
		result.recordSources(source, fillMissingFields(&result.Candidates[index], fallback))
	}
}

func (result *Result) recordSources(source string, fields []string) {
	for _, field := range fields {
		if _, recorded := result.Sources[field]; !recorded {
			result.Sources[field] = source
		}
	}
}

// fillMissingFields copies fields target lacks from fallback and reports
// which fields it filled.
func fillMissingFields(target *Candidate, fallback Candidate) []string {
	var filled []string
	fillText := func(field string, targetValue *string, fallbackValue string) {
		if *targetValue != "" || fallbackValue == "" {
			return
		}
		*targetValue = fallbackValue
		filled = append(filled, field)
	}
	fillText(FieldName, &target.Name, fallback.Name)
	fillText(FieldDescription, &target.Description, fallback.Description)
	fillText(FieldBrand, &target.Brand, fallback.Brand)
	fillText(FieldModel, &target.Model, fallback.Model)
	fillText(FieldSKU, &target.SKU, fallback.SKU)
	if len(target.ImageURLs) == 0 && len(fallback.ImageURLs) > 0 {
		target.ImageURLs = slices.Clone(fallback.ImageURLs)
		filled = append(filled, FieldImages)
	}
	canFillOffers := !hasPriceInformation(target.Offers) && hasPriceInformation(fallback.Offers)
	if canFillOffers {
		target.Offers = append(slices.Clone(target.Offers), fallback.Offers...)
	}
	if canFillOffers && hasSinglePrice(fallback.Offers) {
		filled = append(filled, FieldPrice)
	}
	return filled
}

func hasSinglePrice(offers []Offer) bool {
	return slices.ContainsFunc(offers, func(offer Offer) bool {
		return offer.Price != nil
	})
}

// An aggregate offer carries a price range, which counts as price
// information so that no single price is filled in from a later source.
func hasPriceInformation(offers []Offer) bool {
	return slices.ContainsFunc(offers, func(offer Offer) bool {
		return offer.Price != nil || offer.isAggregate
	})
}

func isPartial(candidates []Candidate) bool {
	if len(candidates) == 0 {
		return true
	}
	return slices.ContainsFunc(candidates, func(candidate Candidate) bool {
		return candidate.Name == "" || len(candidate.ImageURLs) == 0
	})
}

func normalizeCandidates(candidates []Candidate, pageURL *url.URL) []Candidate {
	var normalized []Candidate
	for _, candidate := range candidates {
		if len(normalized) >= maxCandidates {
			break
		}
		clean := normalizeCandidate(candidate, pageURL)
		if isEmptyCandidate(clean) {
			continue
		}
		normalized = append(normalized, clean)
	}
	return normalized
}

func normalizeCandidate(candidate Candidate, pageURL *url.URL) Candidate {
	return Candidate{
		Name:        cleanText(candidate.Name, maxNameRunes),
		Description: cleanText(candidate.Description, maxDescriptionRunes),
		Brand:       cleanText(candidate.Brand, maxFieldRunes),
		Model:       cleanText(candidate.Model, maxFieldRunes),
		SKU:         cleanText(candidate.SKU, maxFieldRunes),
		ImageURLs:   resolveImageURLs(pageURL, candidate.ImageURLs),
		Offers:      normalizeOffers(candidate.Offers),
	}
}

func isEmptyCandidate(candidate Candidate) bool {
	hasText := candidate.Name != "" || candidate.Description != "" || candidate.Brand != "" ||
		candidate.Model != "" || candidate.SKU != ""
	return !hasText && len(candidate.ImageURLs) == 0 && len(candidate.Offers) == 0
}

func normalizeOffers(offers []Offer) []Offer {
	var normalized []Offer
	for _, offer := range offers {
		if len(normalized) >= maxOffersPerCandidate {
			break
		}
		offer.Availability = cleanText(availabilityName(offer.Availability), maxFieldRunes)
		offer.Seller = cleanText(offer.Seller, maxFieldRunes)
		isEmpty := offer.Price == nil && !offer.isAggregate && offer.Availability == "" && offer.Seller == ""
		if isEmpty {
			continue
		}
		normalized = append(normalized, offer)
	}
	return normalized
}

// availabilityName shortens "https://schema.org/InStock" to "InStock".
func availabilityName(availability string) string {
	trimmed := strings.TrimSpace(availability)
	return trimmed[strings.LastIndex(trimmed, "/")+1:]
}

func resolveImageURLs(pageURL *url.URL, rawURLs []string) []string {
	var resolved []string
	for _, rawURL := range rawURLs {
		if len(resolved) >= maxImageURLs {
			break
		}
		imageURL, isUsable := resolveImageURL(pageURL, rawURL)
		if !isUsable || slices.Contains(resolved, imageURL) {
			continue
		}
		resolved = append(resolved, imageURL)
	}
	return resolved
}

func resolveImageURL(pageURL *url.URL, rawURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" || len(trimmed) > maxImageURLBytes {
		return "", false
	}
	reference, err := url.Parse(trimmed)
	if err != nil {
		return "", false
	}
	resolved := pageURL.ResolveReference(reference)
	isSecureHost := resolved.Scheme == secureScheme && resolved.Host != "" && resolved.User == nil
	if !isSecureHost {
		return "", false
	}
	return resolved.String(), true
}
