package extract

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

const (
	// Each pass decodes one layer of entities, so "&amp;lt;b&amp;gt;" needs
	// several passes before the revealed tag can be stripped.
	maxMarkupStrippingPasses = 4
	maxRawTextBytes          = 64 << 10
)

var hiddenTextElements = map[string]bool{
	"script":   true,
	"style":    true,
	"template": true,
	"noscript": true,
}

// cleanText turns untrusted page content into plain text: tags removed,
// entities decoded, whitespace collapsed and length bounded.
func cleanText(raw string, maxRunes int) string {
	text := strings.ToValidUTF8(raw, "")
	for pass := 0; pass <= maxMarkupStrippingPasses; pass++ {
		stripped := stripMarkup(text)
		if stripped == text {
			return truncateRunes(collapseWhitespace(text), maxRunes)
		}
		text = stripped
	}
	return ""
}

func stripMarkup(text string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(text))
	var builder strings.Builder
	hiddenElement := ""
	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			return builder.String()
		case html.TextToken:
			if hiddenElement == "" {
				builder.Write(tokenizer.Text())
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := tokenizer.TagName()
			builder.WriteByte(' ')
			if hiddenElement == "" && hiddenTextElements[string(name)] && tokenType == html.StartTagToken {
				hiddenElement = string(name)
			}
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			builder.WriteByte(' ')
			if string(name) == hiddenElement {
				hiddenElement = ""
			}
		}
	}
}

func collapseWhitespace(text string) string {
	withoutControls := strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return ' '
		}
		return character
	}, text)
	return strings.Join(strings.Fields(withoutControls), " ")
}

func truncateRunes(text string, maxRunes int) string {
	if utf8.RuneCountInString(text) <= maxRunes {
		return text
	}
	runeCount := 0
	for byteIndex := range text {
		if runeCount == maxRunes {
			return strings.TrimSpace(text[:byteIndex])
		}
		runeCount++
	}
	return text
}

// nodeText returns the raw visible text below node; callers still clean it,
// because text nodes can carry entity-encoded markup.
func nodeText(root *html.Node) string {
	var builder strings.Builder
	walkNodes(root, func(node *html.Node) bool {
		if builder.Len() >= maxRawTextBytes {
			return false
		}
		if node.Type == html.TextNode {
			builder.WriteString(node.Data)
			return false
		}
		if node.Type == html.ElementNode {
			builder.WriteByte(' ')
			return !hiddenTextElements[node.Data]
		}
		return true
	})
	text := builder.String()
	if len(text) > maxRawTextBytes {
		text = text[:maxRawTextBytes]
	}
	return text
}
