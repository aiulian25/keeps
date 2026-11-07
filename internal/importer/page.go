package importer

import (
	"context"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/safefetch"
	"github.com/aiulian25/keeps/internal/store"
)

// Limits for page data sent by the browser button. The extractor applies its
// own stricter limits afterwards.
const (
	PageSnapshotBodyLimit      = 4 << 20
	maximumStructuredBlocks    = 20
	maximumStructuredBlockSize = 1 << 20
	maximumMetaTags            = 200
	maximumMetaValueLength     = 4000
	maximumTitleLength         = 500
)

// metaPrefixes are the only page meta tags kept; everything else is dropped.
var metaPrefixes = []string{"og:", "product:", "twitter:"}

const descriptionMetaKey = "description"

type MetaTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PageSnapshot is what the "Add to Keeps" button reads from the product page
// the user is already viewing in their own browser. It is untrusted page
// content and is parsed by the same hardened extractor as fetched pages.
type PageSnapshot struct {
	URL            string    `json:"url"`
	Title          string    `json:"title"`
	MetaTags       []MetaTag `json:"metaTags"`
	StructuredData []string  `json:"structuredData"`
}

var errSnapshotTooLarge = httpapi.NewError(http.StatusRequestEntityTooLarge, "page_too_large", "That page has too much data to read. Add the item manually.")

func keepMeta(key string) bool {
	if key == descriptionMetaKey {
		return true
	}
	for _, prefix := range metaPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func clip(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return strings.ToValidUTF8(value[:limit], "")
}

// buildDocument turns a snapshot back into a minimal HTML page. Attribute
// values are escaped, and "</" inside structured data is written as the
// equivalent JSON escape "<\/" so the data can never close its script tag.
func buildDocument(snapshot PageSnapshot) (string, error) {
	if len(snapshot.StructuredData) > maximumStructuredBlocks || len(snapshot.MetaTags) > maximumMetaTags {
		return "", errSnapshotTooLarge
	}
	var document strings.Builder
	document.WriteString("<!doctype html><html><head><title>")
	document.WriteString(html.EscapeString(clip(snapshot.Title, maximumTitleLength)))
	document.WriteString("</title>")
	for _, tag := range snapshot.MetaTags {
		key := strings.ToLower(strings.TrimSpace(tag.Key))
		if !keepMeta(key) {
			continue
		}
		attribute := "property"
		if key == descriptionMetaKey {
			attribute = "name"
		}
		document.WriteString("<meta " + attribute + `="` + html.EscapeString(key) + `" content="` + html.EscapeString(clip(tag.Value, maximumMetaValueLength)) + `">`)
	}
	for _, block := range snapshot.StructuredData {
		if len(block) > maximumStructuredBlockSize {
			return "", errSnapshotTooLarge
		}
		document.WriteString(`<script type="application/ld+json">`)
		document.WriteString(strings.ReplaceAll(block, "</", `<\/`))
		document.WriteString("</script>")
	}
	document.WriteString("</head><body></body></html>")
	return document.String(), nil
}

// StartFromPage queues an import from page data the user's browser already
// has, so no request is made to the shop for the page itself.
func (s *Service) StartFromPage(ctx context.Context, householdID int64, userID int64, snapshot PageSnapshot) (Draft, error) {
	normalised, err := safefetch.NormaliseURL(snapshot.URL)
	if err != nil {
		code := fetchErrorCode(err)
		return Draft{}, &httpapi.Error{Status: http.StatusBadRequest, Code: code, Message: "This page's address can't be used.", Fields: map[string]string{"url": code}}
	}
	document, err := buildDocument(snapshot)
	if err != nil {
		return Draft{}, err
	}
	identifier, err := newIdentifier()
	if err != nil {
		return Draft{}, err
	}
	now := time.Now()
	_, err = s.store.Database.ExecContext(ctx, `INSERT INTO imports (id, household_id, user_id, source_url, state, provided_document, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		identifier, householdID, userID, normalised.String(), StateQueued, document,
		store.FormatTime(now.Add(config.ImportDraftLifetime)), store.FormatTime(now), store.FormatTime(now))
	if err != nil {
		return Draft{}, err
	}
	s.signal()
	return Draft{ID: identifier, State: StateQueued, SourceURL: normalised.String(), SourceHost: normalised.Hostname()}, nil
}
