package extract

import (
	"strings"

	"golang.org/x/net/html"
)

type pageMetadata struct {
	openGraphTitle       string
	openGraphDescription string
	openGraphImages      []string
	priceAmount          string
	priceCurrency        string
	documentTitle        string
	metaDescription      string
}

func extractPageMetadata(root *html.Node) pageMetadata {
	metadata := pageMetadata{}
	walkElements(root, func(node *html.Node) bool {
		switch node.Data {
		case "title":
			setIfEmpty(&metadata.documentTitle, nodeText(node))
			return false
		case "meta":
			metadata.addMeta(node)
			return false
		case "svg", "math":
			return false
		}
		return true
	})
	return metadata
}

func (metadata *pageMetadata) addMeta(node *html.Node) {
	key := attribute(node, "property")
	if key == "" {
		key = attribute(node, "name")
	}
	content := attribute(node, "content")
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "og:title":
		setIfEmpty(&metadata.openGraphTitle, content)
	case "og:description":
		setIfEmpty(&metadata.openGraphDescription, content)
	case "og:image", "og:image:url", "og:image:secure_url":
		if len(metadata.openGraphImages) < maxRawImageURLs {
			metadata.openGraphImages = append(metadata.openGraphImages, content)
		}
	case "product:price:amount", "og:price:amount":
		setIfEmpty(&metadata.priceAmount, content)
	case "product:price:currency", "og:price:currency":
		setIfEmpty(&metadata.priceCurrency, content)
	case "description":
		setIfEmpty(&metadata.metaDescription, content)
	}
}

func setIfEmpty(target *string, value string) {
	if *target == "" {
		*target = value
	}
}

func (metadata pageMetadata) openGraphCandidate() Candidate {
	return Candidate{
		Name:        metadata.openGraphTitle,
		Description: metadata.openGraphDescription,
		ImageURLs:   metadata.openGraphImages,
		Offers:      pricedOffers(metadata.priceAmount, metadata.priceCurrency),
	}
}

func (metadata pageMetadata) htmlCandidate() Candidate {
	return Candidate{
		Name:        metadata.documentTitle,
		Description: metadata.metaDescription,
	}
}
