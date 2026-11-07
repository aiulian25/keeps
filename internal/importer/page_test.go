package importer

import (
	"context"
	"strings"
	"testing"
)

func TestImportFromBrowserSnapshotNeverFetchesThePage(t *testing.T) {
	fetcher := &fakeFetcher{pages: map[string][]byte{"https://i.ebayimg.com/images/tv.png": pngImage()}}
	service, _ := newTestService(t, fetcher)
	ctx := context.Background()

	snapshot := PageSnapshot{
		URL:   "https://www.ebay.co.uk/itm/137533891783",
		Title: "Sony BRAVIA 3 | eBay",
		MetaTags: []MetaTag{
			{Key: "og:image", Value: "https://i.ebayimg.com/images/tv.png"},
			{Key: "og:title", Value: "Fallback title"},
			{Key: "http-equiv", Value: "refresh"},
		},
		StructuredData: []string{`{"@context":"https://schema.org","@type":"Product","name":"Sony BRAVIA 3 55\" </script><script>alert(1)</script>","brand":{"@type":"Brand","name":"Sony"},"offers":{"@type":"Offer","price":"599.00","priceCurrency":"GBP"}}`},
	}
	pageDraft, err := service.StartFromPage(ctx, 1, 1, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !service.processNext(ctx) {
		t.Fatal("snapshot import not processed")
	}
	loaded, _ := service.Get(ctx, 1, pageDraft.ID)
	if loaded.State != StateDone || len(loaded.Candidates) != 1 {
		t.Fatalf("draft: %+v", loaded)
	}
	candidate := loaded.Candidates[0]
	if candidate.ListingPrice == nil || candidate.ListingPrice.Amount != "599.00" || candidate.Brand != "Sony" {
		t.Fatalf("candidate: %+v", candidate)
	}
	if strings.Contains(candidate.Name, "<") {
		t.Fatalf("markup survived in name: %q", candidate.Name)
	}
	for _, requested := range fetcher.requests {
		if strings.Contains(requested, "ebay.co.uk") {
			t.Fatalf("the shop page or its robots.txt was fetched: %s", requested)
		}
	}
	if len(candidate.ImageIDs) != 1 {
		t.Fatalf("photo not downloaded through the safe fetcher: %v", fetcher.requests)
	}
}

func TestSnapshotDocumentIsSafeAndBounded(t *testing.T) {
	document, err := buildDocument(PageSnapshot{
		Title:          `</title><script>alert(1)</script>`,
		MetaTags:       []MetaTag{{Key: "og:title", Value: `"><script>x</script>`}, {Key: "refresh", Value: "0;url=https://evil"}},
		StructuredData: []string{`{"name":"a</script><img src=x onerror=alert(1)>"}`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(document, "<script") != 1 || strings.Contains(document, "refresh") || strings.Contains(document, "</script><img") {
		t.Fatalf("unsafe document: %s", document)
	}
	tooMany := PageSnapshot{StructuredData: make([]string, maximumStructuredBlocks+1)}
	if _, err := buildDocument(tooMany); err == nil {
		t.Fatal("too many structured data blocks accepted")
	}
	if _, err := (&Service{}).StartFromPage(context.Background(), 1, 1, PageSnapshot{URL: "http://192.168.10.1/x"}); err == nil {
		t.Fatal("non-public page address accepted")
	}
}
