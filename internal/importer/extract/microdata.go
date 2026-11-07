package extract

import (
	"strings"

	"golang.org/x/net/html"
)

const (
	productItemTypeSuffix        = "schema.org/Product"
	aggregateOfferItemTypeSuffix = "schema.org/AggregateOffer"
)

// Elements whose microdata value lives in a URL attribute rather than text.
var microdataValueAttributes = map[string]string{
	"img": "src", "audio": "src", "video": "src", "source": "src", "embed": "src", "iframe": "src",
	"a": "href", "link": "href", "area": "href",
	"data": "value", "meter": "value",
	"time": "datetime",
}

type microdataProperties map[string][]*html.Node

func extractMicrodata(root *html.Node) []Candidate {
	var candidates []Candidate
	walkElements(root, func(node *html.Node) bool {
		if len(candidates) >= maxCandidates {
			return false
		}
		if !hasItemType(node, productItemTypeSuffix) {
			return true
		}
		candidates = append(candidates, productFromMicrodata(node))
		return false
	})
	return candidates
}

func hasItemType(node *html.Node, suffix string) bool {
	if !hasAttribute(node, "itemscope") {
		return false
	}
	for _, itemType := range strings.Fields(attribute(node, "itemtype")) {
		if strings.HasSuffix(itemType, suffix) {
			return true
		}
	}
	return false
}

// collectMicrodataProperties gathers itemprop elements belonging to scope,
// without entering nested item scopes (those belong to their own item).
func collectMicrodataProperties(scope *html.Node) microdataProperties {
	properties := microdataProperties{}
	walkElements(scope, func(node *html.Node) bool {
		for _, name := range strings.Fields(attribute(node, "itemprop")) {
			properties[name] = append(properties[name], node)
		}
		return !hasAttribute(node, "itemscope")
	})
	return properties
}

func (properties microdataProperties) first(name string) string {
	nodes := properties[name]
	if len(nodes) == 0 {
		return ""
	}
	return microdataValue(nodes[0])
}

// firstName reads a property that may be a nested item (brand, seller) or text.
func (properties microdataProperties) firstName(name string) string {
	nodes := properties[name]
	if len(nodes) == 0 {
		return ""
	}
	if hasAttribute(nodes[0], "itemscope") {
		return collectMicrodataProperties(nodes[0]).first("name")
	}
	return microdataValue(nodes[0])
}

func microdataValue(node *html.Node) string {
	if hasAttribute(node, "content") {
		return attribute(node, "content")
	}
	if valueAttribute, usesAttribute := microdataValueAttributes[node.Data]; usesAttribute {
		return attribute(node, valueAttribute)
	}
	return nodeText(node)
}

func productFromMicrodata(scope *html.Node) Candidate {
	properties := collectMicrodataProperties(scope)
	candidate := Candidate{
		Name:        properties.first("name"),
		Description: properties.first("description"),
		Brand:       properties.firstName("brand"),
		Model:       properties.firstName("model"),
		SKU:         properties.first("sku"),
		Offers:      pricedOffers(properties.first("price"), properties.first("priceCurrency")),
	}
	for _, imageNode := range properties["image"] {
		if len(candidate.ImageURLs) >= maxRawImageURLs {
			break
		}
		candidate.ImageURLs = append(candidate.ImageURLs, microdataValue(imageNode))
	}
	for _, offerNode := range properties["offers"] {
		if len(candidate.Offers) >= maxOffersPerCandidate {
			break
		}
		candidate.Offers = append(candidate.Offers, offerFromMicrodata(offerNode))
	}
	return candidate
}

func offerFromMicrodata(node *html.Node) Offer {
	properties := collectMicrodataProperties(node)
	offer := Offer{
		Availability: properties.first("availability"),
		Seller:       properties.firstName("seller"),
	}
	if hasItemType(node, aggregateOfferItemTypeSuffix) {
		offer.isAggregate = true
		return offer
	}
	offer.Price = priceOrNil(properties.first("price"), properties.first("priceCurrency"))
	return offer
}
