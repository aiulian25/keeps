package importer

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aiulian25/keeps/internal/attachments"
	"github.com/aiulian25/keeps/internal/blob"
	"github.com/aiulian25/keeps/internal/importer/extract"
	"github.com/aiulian25/keeps/internal/items"
	"github.com/aiulian25/keeps/internal/safefetch"
	"github.com/aiulian25/keeps/internal/store"
)

type fakeFetcher struct {
	pages    map[string][]byte
	failures map[string]error
	requests []string
}

func (f *fakeFetcher) Get(ctx context.Context, rawURL string, limits safefetch.Limits, accept string) (*safefetch.Response, error) {
	f.requests = append(f.requests, rawURL)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err, failed := f.failures[rawURL]; failed {
		return nil, err
	}
	body, found := f.pages[rawURL]
	if !found {
		return nil, &safefetch.Error{Code: safefetch.CodeNotFound}
	}
	final, _ := url.Parse(rawURL)
	return &safefetch.Response{FinalURL: final, StatusCode: 200, Body: body}, nil
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "imports", name))
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func newTestService(t *testing.T, fetcher Fetcher) (*Service, *store.Store) {
	t.Helper()
	directory := t.TempDir()
	database, err := store.Open(context.Background(), filepath.Join(directory, "keeps.db"), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	database.Database.Exec(`INSERT INTO households (id, name, created_at) VALUES (1, 'Home', 'now')`)
	database.Database.Exec(`INSERT INTO users (id, household_id, username, display_name, password_hash, webauthn_handle, created_at, updated_at) VALUES (1, 1, 'a', 'a', 'x', x'01', 'now', 'now')`)
	blobs, _ := blob.New(filepath.Join(directory, "blobs"), filepath.Join(directory, "tmp"))
	attachmentService := attachments.NewService(database, blobs, directory)
	return NewService(database, fetcher, attachmentService, items.NewService(database)), database
}

func pngImage() []byte {
	var buffer bytes.Buffer
	png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	return buffer.Bytes()
}

func TestImportReviewAndSaveKeepsListingSeparateFromPaid(t *testing.T) {
	page := fixture(t, "jsonld-product.html")
	fetcher := &fakeFetcher{pages: map[string][]byte{"https://shop.example/p/1": page}}
	extractedImages := imageURLs(t, page, "https://shop.example/p/1")
	for _, imageURL := range extractedImages {
		fetcher.pages[imageURL] = pngImage()
	}
	service, database := newTestService(t, fetcher)
	ctx := context.Background()

	draft, err := service.Start(ctx, 1, 1, "https://shop.example/p/1#reviews")
	if err != nil || strings.Contains(draft.SourceURL, "#") {
		t.Fatalf("start: %v %s", err, draft.SourceURL)
	}
	if !service.processNext(ctx) {
		t.Fatal("job not processed")
	}
	loaded, _ := service.Get(ctx, 1, draft.ID)
	if loaded.State != StateDone || len(loaded.Candidates) == 0 || loaded.Candidates[0].ListingPrice == nil {
		t.Fatalf("draft: %+v", loaded)
	}
	candidate := loaded.Candidates[0]
	if len(extractedImages) > 0 && len(candidate.ImageIDs) == 0 {
		t.Fatal("product images not stored")
	}

	var itemCount int
	database.Database.QueryRow(`SELECT count(*) FROM items`).Scan(&itemCount)
	if itemCount != 0 {
		t.Fatal("import saved an item without review")
	}

	input := items.Input{Name: candidate.Name, Description: "My own words", PricePaid: &items.Money{Amount: "79", Currency: "GBP"}, AttachmentIDs: candidate.ImageIDs}
	saved, err := service.Save(ctx, 1, 1, draft.ID, 0, input, "")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Listing == nil || saved.Listing.Price == nil || saved.PricePaid.Amount != "79.00" {
		t.Fatalf("listing/paid not separated: %+v %+v", saved.Listing, saved.PricePaid)
	}
	if saved.PurchaseDate != nil {
		t.Fatal("import time used as purchase date")
	}
	if saved.FieldSources["name"] != "import" || saved.FieldSources["description"] != "user" {
		t.Fatalf("provenance: %v", saved.FieldSources)
	}
}

func imageURLs(t *testing.T, page []byte, pageURL string) []string {
	t.Helper()
	parsed, _ := url.Parse(pageURL)
	var found []string
	for _, candidate := range mustExtract(t, parsed, page).Candidates {
		found = append(found, candidate.ImageURLs...)
	}
	return found
}

func TestImportFailuresAndFallbacks(t *testing.T) {
	login := &safefetch.Error{Code: safefetch.CodeNeedsLogin}
	fetcher := &fakeFetcher{
		pages: map[string][]byte{
			"https://shop.example/title":         fixture(t, "title-only.html"),
			"https://blocked.example/robots.txt": []byte("User-agent: *\nDisallow: /\n"),
			"https://blocked.example/p":          fixture(t, "jsonld-product.html"),
		},
		failures: map[string]error{"https://shop.example/orders/123": login},
	}
	service, _ := newTestService(t, fetcher)
	ctx := context.Background()

	run := func(link string) Draft {
		draft, err := service.Start(ctx, 1, 1, link)
		if err != nil {
			t.Fatalf("%s: %v", link, err)
		}
		service.processNext(ctx)
		loaded, _ := service.Get(ctx, 1, draft.ID)
		return loaded
	}
	if draft := run("https://shop.example/orders/123"); draft.State != StateFailed || draft.ErrorCode != "needs_login" || draft.SourceURL == "" {
		t.Fatalf("login page: %+v", draft)
	}
	if draft := run("https://blocked.example/p"); draft.ErrorCode != "blocked" {
		t.Fatalf("robots disallow ignored: %+v", draft)
	}
	if draft := run("https://shop.example/title"); draft.State != StateDone || !draft.Partial || draft.Candidates[0].Name == "" {
		t.Fatalf("title-only partial: %+v", draft)
	}

	if _, err := service.Start(ctx, 1, 1, "https://bucket.s3.example/invoice.pdf?X-Amz-Signature=abc&X-Amz-Credential=x"); err == nil || !strings.Contains(err.Error(), "private_document_link") {
		t.Fatalf("signed invoice link not refused: %v", err)
	}
	before := len(fetcher.requests)
	if _, err := service.Start(ctx, 1, 1, "http://192.168.10.1/admin"); err == nil {
		t.Fatal("private address accepted")
	}
	if len(fetcher.requests) != before {
		t.Fatal("refused link was fetched")
	}
}

func TestCancelAndRestartRecovery(t *testing.T) {
	service, database := newTestService(t, &fakeFetcher{pages: map[string][]byte{}})
	ctx := context.Background()
	draft, _ := service.Start(ctx, 1, 1, "https://shop.example/p")
	if err := service.Cancel(ctx, 1, draft.ID); err != nil {
		t.Fatal(err)
	}
	if service.processNext(ctx) {
		t.Fatal("cancelled job was processed")
	}

	interrupted, _ := service.Start(ctx, 1, 1, "https://shop.example/q")
	database.Database.Exec(`UPDATE imports SET state = 'running', attempts = 1 WHERE id = ?`, interrupted.ID)
	service.recoverInterrupted(ctx)
	if loaded, _ := service.Get(ctx, 1, interrupted.ID); loaded.State != StateQueued {
		t.Fatalf("interrupted job not retryable: %s", loaded.State)
	}
	database.Database.Exec(`UPDATE imports SET state = 'running', attempts = 2 WHERE id = ?`, interrupted.ID)
	service.recoverInterrupted(ctx)
	if loaded, _ := service.Get(ctx, 1, interrupted.ID); loaded.State != StateFailed || loaded.ErrorCode != "timeout" {
		t.Fatalf("exhausted job not failed: %+v", loaded)
	}
	if _, err := service.Get(ctx, 2, interrupted.ID); err == nil {
		t.Fatal("another household could read the import")
	}
}

func mustExtract(t *testing.T, pageURL *url.URL, page []byte) extract.Result {
	t.Helper()
	result, err := extract.Extract(pageURL, page)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestFailedImageIsNotGivenToOtherVariants(t *testing.T) {
	fetcher := &fakeFetcher{pages: map[string][]byte{"https://shop.example/good.png": pngImage(), "https://shop.example/p/2": []byte("<html></html>")}}
	service, database := newTestService(t, fetcher)
	ctx := context.Background()
	draft, err := service.Start(ctx, 1, 1, "https://shop.example/p/2")
	if err != nil {
		t.Fatal(err)
	}
	shared := []string{"https://shop.example/missing.png", "https://shop.example/good.png"}
	extracted := extract.Result{Candidates: []extract.Candidate{{ImageURLs: shared}, {ImageURLs: shared}}}
	candidates := make([]Candidate, 2)
	service.downloadImages(ctx, 1, draft.ID, extracted, candidates)
	for index, candidate := range candidates {
		if len(candidate.ImageIDs) != 1 || candidate.ImageIDs[0] == 0 {
			t.Fatalf("variant %d got %v", index, candidate.ImageIDs)
		}
	}
	if len(fetcher.requests) != 2 {
		t.Fatalf("each distinct image should be tried once: %v", fetcher.requests)
	}

	legacy, _ := json.Marshal(Result{Candidates: []Candidate{{Name: "Old draft", ImageIDs: []int64{0, candidates[0].ImageIDs[0]}}}})
	database.Database.Exec(`UPDATE imports SET state = ?, result_json = ? WHERE id = ?`, StateDone, string(legacy), draft.ID)
	loaded, _ := service.Get(ctx, 1, draft.ID)
	if ids := loaded.Candidates[0].ImageIDs; len(ids) != 1 || ids[0] == 0 {
		t.Fatalf("a stored draft still offers a failed image: %v", ids)
	}
	if _, err := service.Save(ctx, 1, 1, draft.ID, 0, items.Input{Name: "Second variant", AttachmentIDs: loaded.Candidates[0].ImageIDs}, ""); err != nil {
		t.Fatalf("second variant could not be saved: %v", err)
	}
}
