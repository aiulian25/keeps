package extract

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	fixtureDirectory = "../../../testdata/imports"
	testPageURL      = "https://shop.example.test/products/item"
)

func extractFixture(t *testing.T, name string) Result {
	t.Helper()
	document, err := os.ReadFile(filepath.Join(fixtureDirectory, name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return extractDocument(t, document)
}

func extractDocument(t *testing.T, document []byte) Result {
	t.Helper()
	pageURL, err := url.Parse(testPageURL)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Extract(pageURL, document)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	return result
}

func onlyCandidate(t *testing.T, result Result) Candidate {
	t.Helper()
	if len(result.Candidates) != 1 {
		t.Fatalf("got %d candidates, want 1: %+v", len(result.Candidates), result.Candidates)
	}
	return result.Candidates[0]
}

func assertEqual[Value any](t *testing.T, label string, got, want Value) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %#v, want %#v", label, got, want)
	}
}

func assertPrice(t *testing.T, offer Offer, minorUnits int64, currency string) {
	t.Helper()
	if offer.Price == nil {
		t.Fatalf("offer has no price, want %d %s", minorUnits, currency)
	}
	assertEqual(t, "price", *offer.Price, Money{MinorUnits: minorUnits, Currency: currency})
}

func TestJSONLDProduct(t *testing.T) {
	result := extractFixture(t, "jsonld-product.html")
	candidate := onlyCandidate(t, result)
	assertEqual(t, "name", candidate.Name, "Aurora Desk Lamp")
	assertEqual(t, "description", candidate.Description, "Adjustable LED desk lamp with & warm-white mode.")
	assertEqual(t, "brand", candidate.Brand, "Lumen & Co")
	assertEqual(t, "model", candidate.Model, "AL-200")
	assertEqual(t, "sku", candidate.SKU, "LAMP-200-BLK")
	assertEqual(t, "images", candidate.ImageURLs, []string{
		"https://shop.example.test/images/lamp-front.jpg",
		"https://cdn.example.test/lamp-side.jpg",
	})
	if len(candidate.Offers) != 1 {
		t.Fatalf("offers = %+v", candidate.Offers)
	}
	assertPrice(t, candidate.Offers[0], 129999, "GBP")
	assertEqual(t, "availability", candidate.Offers[0].Availability, "InStock")
	assertEqual(t, "seller", candidate.Offers[0].Seller, "Example Lighting")
	assertEqual(t, "page title", result.PageTitle, "Aurora Desk Lamp | Example Lighting")
	assertEqual(t, "sources", result.Sources, map[string]string{
		FieldName: SourceJSONLD, FieldDescription: SourceJSONLD, FieldBrand: SourceJSONLD,
		FieldModel: SourceJSONLD, FieldSKU: SourceJSONLD, FieldImages: SourceJSONLD, FieldPrice: SourceJSONLD,
	})
	assertEqual(t, "partial", result.Partial, false)
}

func TestJSONLDGraph(t *testing.T) {
	result := extractFixture(t, "jsonld-graph.html")
	candidate := onlyCandidate(t, result)
	assertEqual(t, "name", candidate.Name, "Trail Bottle 750ml")
	assertEqual(t, "images", candidate.ImageURLs, []string{"https://cdn.example.test/bottle.jpg"})
	assertPrice(t, candidate.Offers[0], 2450, "USD")
	assertEqual(t, "sources", result.Sources, map[string]string{
		FieldName: SourceJSONLD, FieldImages: SourceJSONLD, FieldPrice: SourceJSONLD,
	})
}

func TestJSONLDArrayTypesAndOffers(t *testing.T) {
	result := extractFixture(t, "jsonld-array-types.html")
	candidate := onlyCandidate(t, result)
	assertEqual(t, "name", candidate.Name, "Field Camera X1")
	assertEqual(t, "brand", candidate.Brand, "Example Optics")
	if len(candidate.Offers) != 2 {
		t.Fatalf("offers = %+v", candidate.Offers)
	}
	assertPrice(t, candidate.Offers[0], 49900, "EUR")
	assertEqual(t, "first seller", candidate.Offers[0].Seller, "Shop One")
	assertPrice(t, candidate.Offers[1], 47900, "EUR")
	assertEqual(t, "second seller", candidate.Offers[1].Seller, "Shop Two")
	assertEqual(t, "second availability", candidate.Offers[1].Availability, "OutOfStock")
}

func TestJSONLDVariantsBecomeCandidates(t *testing.T) {
	result := extractFixture(t, "jsonld-variants.html")
	if len(result.Candidates) != 3 {
		t.Fatalf("got %d candidates, want 3", len(result.Candidates))
	}
	wantNames := []string{"Merino Tee - Small", "Merino Tee - Medium", "Merino Tee - Large"}
	wantSKUs := []string{"TEE-S", "TEE-M", "TEE-L"}
	wantPrices := []int64{4000, 4200, 4500}
	for index, candidate := range result.Candidates {
		assertEqual(t, "name", candidate.Name, wantNames[index])
		assertEqual(t, "sku", candidate.SKU, wantSKUs[index])
		assertPrice(t, candidate.Offers[0], wantPrices[index], "GBP")
		assertEqual(t, "inherited brand", candidate.Brand, "Fellside")
		assertEqual(t, "inherited description", candidate.Description, "Lightweight merino wool t-shirt.")
		assertEqual(t, "inherited images", candidate.ImageURLs, []string{"https://cdn.example.test/tee.jpg"})
	}
	assertEqual(t, "price source", result.Sources[FieldPrice], SourceJSONLD)
	assertEqual(t, "partial", result.Partial, false)
}

func TestJSONLDAggregateOfferSetsNoSinglePrice(t *testing.T) {
	result := extractFixture(t, "jsonld-aggregate-offer.html")
	candidate := onlyCandidate(t, result)
	assertEqual(t, "name", candidate.Name, "Cast Iron Pan 26cm")
	for _, offer := range candidate.Offers {
		if offer.Price != nil {
			t.Errorf("aggregate offer produced a single price %+v", *offer.Price)
		}
	}
	if source, found := result.Sources[FieldPrice]; found {
		t.Errorf("price source recorded as %q, want none", source)
	}
}

func TestMicrodataProduct(t *testing.T) {
	result := extractFixture(t, "microdata-product.html")
	candidate := onlyCandidate(t, result)
	assertEqual(t, "name", candidate.Name, "Oak Stool")
	assertEqual(t, "description", candidate.Description, "Solid oak three-legged stool.")
	assertEqual(t, "brand", candidate.Brand, "Woodshop")
	assertEqual(t, "sku", candidate.SKU, "STOOL-OAK")
	assertEqual(t, "images", candidate.ImageURLs, []string{"https://shop.example.test/img/stool.jpg"})
	assertPrice(t, candidate.Offers[0], 8950, "GBP")
	assertEqual(t, "availability", candidate.Offers[0].Availability, "InStock")
	assertEqual(t, "sources", result.Sources, map[string]string{
		FieldName: SourceMicrodata, FieldDescription: SourceMicrodata, FieldBrand: SourceMicrodata,
		FieldSKU: SourceMicrodata, FieldImages: SourceMicrodata, FieldPrice: SourceMicrodata,
	})
}

func TestOpenGraphOnly(t *testing.T) {
	result := extractFixture(t, "opengraph-only.html")
	candidate := onlyCandidate(t, result)
	assertEqual(t, "name", candidate.Name, "Canvas Tote Bag")
	assertEqual(t, "description", candidate.Description, "Heavy canvas tote with inner pocket.")
	assertEqual(t, "images", candidate.ImageURLs, []string{"https://cdn.example.test/tote.jpg"})
	assertPrice(t, candidate.Offers[0], 1500, "GBP")
	assertEqual(t, "sources", result.Sources, map[string]string{
		FieldName: SourceOpenGraph, FieldDescription: SourceOpenGraph, FieldImages: SourceOpenGraph, FieldPrice: SourceOpenGraph,
	})
	assertEqual(t, "page title", result.PageTitle, "Canvas Tote Bag – Example Store")
	assertEqual(t, "partial", result.Partial, false)
}

func TestTitleOnlyIsPartial(t *testing.T) {
	result := extractFixture(t, "title-only.html")
	candidate := onlyCandidate(t, result)
	assertEqual(t, "name", candidate.Name, "Ceramic Mug – Example Pottery")
	assertEqual(t, "page title", result.PageTitle, "Ceramic Mug – Example Pottery")
	assertEqual(t, "sources", result.Sources, map[string]string{FieldName: SourceHTML})
	assertEqual(t, "partial", result.Partial, true)
}

func TestMalformedDocumentFallsBack(t *testing.T) {
	result := extractFixture(t, "malformed.html")
	candidate := onlyCandidate(t, result)
	assertEqual(t, "name", candidate.Name, "Recovered Kettle")
	assertEqual(t, "images", candidate.ImageURLs, []string{"https://cdn.example.test/kettle.jpg"})
	assertEqual(t, "name source", result.Sources[FieldName], SourceOpenGraph)
}

func TestDeepNestingBlockIsSkipped(t *testing.T) {
	result := extractFixture(t, "deep-nesting.html")
	candidate := onlyCandidate(t, result)
	assertEqual(t, "name", candidate.Name, "Shallow Fallback")
	assertEqual(t, "name source", result.Sources[FieldName], SourceOpenGraph)
	assertEqual(t, "partial", result.Partial, true)
}

func TestCurrencyExponents(t *testing.T) {
	fixtures := []struct {
		name       string
		minorUnits int64
		currency   string
	}{
		{"currency-jpy.html", 12800, "JPY"},
		{"currency-bhd.html", 12345, "BHD"},
		{"eu-price.html", 129999, "EUR"},
	}
	for _, fixture := range fixtures {
		candidate := onlyCandidate(t, extractFixture(t, fixture.name))
		assertPrice(t, candidate.Offers[0], fixture.minorUnits, fixture.currency)
	}
}

func TestHostileMarkupBecomesPlainText(t *testing.T) {
	result := extractFixture(t, "xss.html")
	candidate := onlyCandidate(t, result)
	assertEqual(t, "name", candidate.Name, "Gadget Pro")
	assertEqual(t, "description", candidate.Description, "Bold claim end")
	assertEqual(t, "brand", candidate.Brand, "Acme")
	assertEqual(t, "images", candidate.ImageURLs, []string{"https://cdn.example.test/gadget.jpg"})
	for _, field := range []string{candidate.Name, candidate.Description, candidate.Brand} {
		if strings.Contains(field, "alert") || strings.Contains(field, "javascript:") {
			t.Errorf("hostile content survived: %q", field)
		}
	}
}

func TestNoFixtureFieldContainsMarkup(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.Join(fixtureDirectory, "*.html"))
	if err != nil || len(fixtures) == 0 {
		t.Fatalf("no fixtures found: %v", err)
	}
	for _, fixturePath := range fixtures {
		result := extractFixture(t, filepath.Base(fixturePath))
		fields := []string{result.PageTitle}
		for _, candidate := range result.Candidates {
			fields = append(fields, candidate.Name, candidate.Description, candidate.Brand, candidate.Model, candidate.SKU)
			for _, offer := range candidate.Offers {
				fields = append(fields, offer.Availability, offer.Seller)
			}
			for _, imageURL := range candidate.ImageURLs {
				if !strings.HasPrefix(imageURL, "https://") {
					t.Errorf("%s: non-https image %q", fixturePath, imageURL)
				}
			}
		}
		for _, field := range fields {
			if strings.ContainsAny(field, "<>") {
				t.Errorf("%s: field contains markup: %q", fixturePath, field)
			}
		}
	}
}

func TestOversizedJSONLDBlockIsSkipped(t *testing.T) {
	padding := strings.Repeat("a", maxJSONLDBlockBytes)
	document := fmt.Sprintf(`<html><head><title>Fallback Title</title><script type="application/ld+json">
{"@type":"Product","name":"Huge","description":"%s"}</script></head></html>`, padding)
	result := extractDocument(t, []byte(document))
	candidate := onlyCandidate(t, result)
	assertEqual(t, "name", candidate.Name, "Fallback Title")
	assertEqual(t, "name source", result.Sources[FieldName], SourceHTML)
}

func TestJSONLDBlockLimit(t *testing.T) {
	var document strings.Builder
	document.WriteString("<html><head><title>Fallback Title</title>")
	for range maxJSONLDBlocks {
		document.WriteString(`<script type="application/ld+json">{"@type":"WebPage"}</script>`)
	}
	document.WriteString(`<script type="application/ld+json">{"@type":"Product","name":"Too Late"}</script></head></html>`)
	candidate := onlyCandidate(t, extractDocument(t, []byte(document.String())))
	assertEqual(t, "name", candidate.Name, "Fallback Title")
}

func TestCandidateLimit(t *testing.T) {
	var products []string
	for index := range maxCandidates + 5 {
		products = append(products, fmt.Sprintf(`{"@type":"Product","name":"Item %d"}`, index))
	}
	document := `<script type="application/ld+json">{"@graph":[` + strings.Join(products, ",") + `]}</script>`
	result := extractDocument(t, []byte(document))
	assertEqual(t, "candidate count", len(result.Candidates), maxCandidates)
}

func TestImageLimitAndNameTruncation(t *testing.T) {
	var images []string
	for index := range maxImageURLs + 3 {
		images = append(images, fmt.Sprintf(`"/image-%d.jpg"`, index))
	}
	longName := strings.Repeat("é", maxNameRunes+100)
	document := `<script type="application/ld+json">{"@type":"Product","name":"` + longName +
		`","image":[` + strings.Join(images, ",") + `]}</script>`
	candidate := onlyCandidate(t, extractDocument(t, []byte(document)))
	assertEqual(t, "image count", len(candidate.ImageURLs), maxImageURLs)
	assertEqual(t, "name", candidate.Name, strings.Repeat("é", maxNameRunes))
}

func TestDocumentIsTruncated(t *testing.T) {
	padding := strings.Repeat(" ", maxDocumentBytes)
	document := `<html><head><title>Early Title</title></head><body>` + padding +
		`<script type="application/ld+json">{"@type":"Product","name":"Beyond Limit"}</script></body></html>`
	candidate := onlyCandidate(t, extractDocument(t, []byte(document)))
	assertEqual(t, "name", candidate.Name, "Early Title")
}

// The HTML parser refuses documents nested beyond its open-element limit;
// that must surface as an error, never a crash.
func TestDeepHTMLNestingReturnsError(t *testing.T) {
	const nestingDepth = 100000
	document := `<title>Nested</title>` + strings.Repeat("<div>", nestingDepth) + "text"
	pageURL, err := url.Parse(testPageURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Extract(pageURL, []byte(document)); err == nil {
		t.Error("deeply nested HTML accepted")
	}
}

func TestRejectsRelativePageURL(t *testing.T) {
	if _, err := Extract(nil, nil); err == nil {
		t.Error("nil page URL accepted")
	}
	if _, err := Extract(&url.URL{Path: "/relative"}, nil); err == nil {
		t.Error("relative page URL accepted")
	}
}
