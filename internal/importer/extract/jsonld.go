package extract

import (
	"encoding/json"
	"errors"
	"io"
	"maps"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

const (
	maxJSONLDBlocks       = 20
	maxJSONLDBlockBytes   = 1 << 20
	maxJSONDepth          = 32
	maxJSONObjectsVisited = 5000
	jsonLDScriptType      = "application/ld+json"
	typeProduct           = "Product"
	typeProductGroup      = "ProductGroup"
	typeAggregateOffer    = "AggregateOffer"
)

type jsonLDWalker struct {
	visited    int
	candidates []Candidate
}

func extractJSONLD(root *html.Node) []Candidate {
	walker := &jsonLDWalker{}
	blocks := 0
	walkElements(root, func(node *html.Node) bool {
		if blocks >= maxJSONLDBlocks {
			return false
		}
		if !isJSONLDScript(node) {
			return true
		}
		blocks++
		walker.parseBlock(nodeRawText(node))
		return false
	})
	return walker.candidates
}

func isJSONLDScript(node *html.Node) bool {
	if node.Data != "script" {
		return false
	}
	scriptType, _, _ := strings.Cut(attribute(node, "type"), ";")
	return strings.EqualFold(strings.TrimSpace(scriptType), jsonLDScriptType)
}

func nodeRawText(node *html.Node) string {
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			builder.WriteString(child.Data)
		}
	}
	return builder.String()
}

func (walker *jsonLDWalker) parseBlock(block string) {
	if len(block) > maxJSONLDBlockBytes || !withinJSONDepth(block) {
		return
	}
	decoder := json.NewDecoder(strings.NewReader(block))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return
	}
	walker.visit(value)
}

// withinJSONDepth scans tokens before decoding so over-deep documents are
// rejected without ever building them in memory.
func withinJSONDepth(block string) bool {
	decoder := json.NewDecoder(strings.NewReader(block))
	depth := 0
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return true
		}
		if err != nil {
			return false
		}
		delimiter, isDelimiter := token.(json.Delim)
		if !isDelimiter {
			continue
		}
		if delimiter == '{' || delimiter == '[' {
			depth++
		} else {
			depth--
		}
		if depth > maxJSONDepth {
			return false
		}
	}
}

func (walker *jsonLDWalker) exhausted() bool {
	return walker.visited > maxJSONObjectsVisited || len(walker.candidates) >= maxCandidates
}

func (walker *jsonLDWalker) visit(value any) {
	if walker.exhausted() {
		return
	}
	switch typed := value.(type) {
	case []any:
		walker.visited++
		for _, item := range typed {
			walker.visit(item)
		}
	case map[string]any:
		walker.visitObject(typed)
	}
}

func (walker *jsonLDWalker) visitObject(object map[string]any) {
	walker.visited++
	if walker.exhausted() {
		return
	}
	if hasType(object, typeProductGroup) {
		walker.addProductGroup(object)
		return
	}
	if hasType(object, typeProduct) {
		walker.candidates = append(walker.candidates, productFromJSONLD(object))
		return
	}
	for _, key := range slices.Sorted(maps.Keys(object)) {
		walker.visit(object[key])
	}
}

func (walker *jsonLDWalker) addProductGroup(group map[string]any) {
	groupCandidate := productFromJSONLD(group)
	addedVariant := false
	for _, variantValue := range asList(group["hasVariant"]) {
		variant, isObject := variantValue.(map[string]any)
		if !isObject {
			continue
		}
		walker.visited++
		if walker.exhausted() {
			return
		}
		candidate := productFromJSONLD(variant)
		fillMissingFields(&candidate, groupCandidate)
		walker.candidates = append(walker.candidates, candidate)
		addedVariant = true
	}
	if !addedVariant {
		walker.candidates = append(walker.candidates, groupCandidate)
	}
}

func productFromJSONLD(object map[string]any) Candidate {
	return Candidate{
		Name:        textValue(object["name"]),
		Description: textValue(object["description"]),
		Brand:       nameValue(object["brand"]),
		Model:       nameValue(object["model"]),
		SKU:         textValue(object["sku"]),
		ImageURLs:   imageValues(object["image"], nil),
		Offers:      offersFromJSONLD(object["offers"]),
	}
}

func offersFromJSONLD(value any) []Offer {
	var offers []Offer
	for _, item := range asList(value) {
		object, isObject := item.(map[string]any)
		if !isObject || len(offers) >= maxOffersPerCandidate {
			continue
		}
		offers = append(offers, offerFromJSONLD(object))
	}
	return offers
}

func offerFromJSONLD(object map[string]any) Offer {
	offer := Offer{
		Availability: textValue(object["availability"]),
		Seller:       nameValue(object["seller"]),
	}
	if hasType(object, typeAggregateOffer) {
		offer.isAggregate = true
		return offer
	}
	amount := textValue(object["price"])
	currency := textValue(object["priceCurrency"])
	specification, hasSpecification := firstObject(object["priceSpecification"])
	if amount == "" && hasSpecification {
		amount = textValue(specification["price"])
		currency = textValue(specification["priceCurrency"])
	}
	offer.Price = priceOrNil(amount, currency)
	return offer
}

func hasType(object map[string]any, wanted string) bool {
	for _, typeValue := range asList(object["@type"]) {
		typeName, isString := typeValue.(string)
		if !isString {
			continue
		}
		// Accept "Product", "schema:Product" and "https://schema.org/Product".
		localName := typeName[strings.LastIndexAny(typeName, "/:")+1:]
		if localName == wanted {
			return true
		}
	}
	return false
}

func asList(value any) []any {
	if value == nil {
		return nil
	}
	if list, isList := value.([]any); isList {
		return list
	}
	return []any{value}
}

func firstObject(value any) (map[string]any, bool) {
	for _, item := range asList(value) {
		if object, isObject := item.(map[string]any); isObject {
			return object, true
		}
	}
	return nil, false
}

func textValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case []any:
		for _, item := range typed {
			if text := textValue(item); text != "" {
				return text
			}
		}
	case map[string]any:
		return textValue(typed["@value"])
	}
	return ""
}

func nameValue(value any) string {
	if object, isObject := firstObject(value); isObject {
		return textValue(object["name"])
	}
	return textValue(value)
}

func imageValues(value any, collected []string) []string {
	switch typed := value.(type) {
	case string:
		if len(collected) < maxRawImageURLs {
			collected = append(collected, typed)
		}
	case []any:
		for _, item := range typed {
			collected = imageValues(item, collected)
		}
	case map[string]any:
		imageURL := textValue(typed["url"])
		if imageURL == "" {
			imageURL = textValue(typed["contentUrl"])
		}
		collected = imageValues(imageURL, collected)
	}
	return collected
}
