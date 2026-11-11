package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

func (c *testClient) uploadInvoice(t *testing.T, name string) int64 {
	t.Helper()
	var body struct {
		ID int64 `json:"id"`
	}
	c.upload("invoice", name, "application/pdf", []byte("%PDF-1.4\n"+name+"\n%%EOF")).json(t, &body)
	return body.ID
}

func purchaseCount(server *testServer) int {
	var count int
	server.application.Store.Database.QueryRow(`SELECT count(*) FROM purchases`).Scan(&count)
	return count
}

func TestEveryInvoiceInOneSaveStaysOnTheItem(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	var photo struct {
		ID int64 `json:"id"`
	}
	client.upload("photo", "front.png", "image/png", pngBytes(10, 10)).json(t, &photo)
	receipt, warranty := client.uploadInvoice(t, "receipt.pdf"), client.uploadInvoice(t, "warranty.pdf")

	created := client.createItem(map[string]any{"name": "Name only", "attachmentIds": []int64{receipt, photo.ID, warranty}})
	var reloaded itemResponse
	client.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d", created.ID), nil).json(t, &reloaded)
	if len(reloaded.Documents) != 2 || len(reloaded.Photos) != 1 || purchaseCount(server) != 1 {
		t.Fatalf("create kept %d invoices and %d photos in %d purchases", len(reloaded.Documents), len(reloaded.Photos), purchaseCount(server))
	}

	plain := client.createItem(map[string]any{"name": "Updated later"})
	first, second := client.uploadInvoice(t, "a.pdf"), client.uploadInvoice(t, "b.pdf")
	if result := client.put(fmt.Sprintf("/api/v1/items/%d", plain.ID), plain.Revision, map[string]any{"name": "Updated later", "attachmentIds": []int64{first, second}}); result.Status != http.StatusOK {
		t.Fatalf("update: %d %s", result.Status, result.Body)
	}
	client.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d", plain.ID), nil).json(t, &reloaded)
	if len(reloaded.Documents) != 2 || purchaseCount(server) != 2 {
		t.Fatalf("update kept %d invoices in %d purchases", len(reloaded.Documents), purchaseCount(server))
	}

	later := client.createItem(map[string]any{"name": "Linked later"})
	third, fourth := client.uploadInvoice(t, "c.pdf"), client.uploadInvoice(t, "d.pdf")
	link := fmt.Sprintf("/api/v1/items/%d/attachments", later.ID)
	for attempt := 0; attempt < 2; attempt++ {
		if result := client.do(http.MethodPost, link, map[string]any{"attachmentIds": []int64{third, fourth}}); result.Status != http.StatusNoContent {
			t.Fatalf("link: %d %s", result.Status, result.Body)
		}
	}
	client.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d", later.ID), nil).json(t, &reloaded)
	if len(reloaded.Documents) != 2 || purchaseCount(server) != 3 {
		t.Fatalf("repeated linking gave %d invoices in %d purchases", len(reloaded.Documents), purchaseCount(server))
	}

	batch := client.createItem(map[string]any{"name": "Bad batch"})
	fifth := client.uploadInvoice(t, "e.pdf")
	if result := client.do(http.MethodPost, fmt.Sprintf("/api/v1/items/%d/attachments", batch.ID), map[string]any{"attachmentIds": []int64{fifth, 999999}}); result.Status == http.StatusNoContent {
		t.Fatal("a batch with a missing file was accepted")
	}
	if purchaseCount(server) != 3 {
		t.Fatal("a rejected batch left a purchase behind")
	}
}

func TestReceiptAddedLaterCountsAsProofOfPurchase(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	item := client.createItem(map[string]any{"name": "Name only"})
	var manual struct {
		ID int64 `json:"id"`
	}
	client.upload("document", "manual.pdf", "application/pdf", []byte("%PDF-1.4\nmanual\n%%EOF")).json(t, &manual)
	client.do(http.MethodPost, fmt.Sprintf("/api/v1/items/%d/attachments", item.ID), map[string]any{"attachmentIds": []int64{manual.ID}})
	if hasReceiptCount(t, client) != 0 {
		t.Fatal("a manual made the item count as having a receipt")
	}

	receipt := client.uploadInvoice(t, "receipt.pdf")
	client.do(http.MethodPost, fmt.Sprintf("/api/v1/items/%d/attachments", item.ID), map[string]any{"attachmentIds": []int64{receipt}})
	if hasReceiptCount(t, client) != 1 {
		t.Fatal("a receipt added later did not count")
	}
	owner := client.facts(t, item.ID)
	sibling := client.createItem(map[string]any{"name": "Same purchase", "purchaseId": owner.PurchaseID})
	var reloaded itemResponse
	client.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d", sibling.ID), nil).json(t, &reloaded)
	if len(reloaded.Documents) != 1 {
		t.Fatalf("an item on the same purchase does not see the late receipt: %+v", reloaded.Documents)
	}
}

func hasReceiptCount(t *testing.T, client *testClient) int {
	t.Helper()
	var page struct {
		Total int `json:"total"`
	}
	client.do(http.MethodGet, "/api/v1/items?hasReceipt=1", nil).json(t, &page)
	return page.Total
}

// heldBody keeps a request waiting at its first read, after the handler has
// looked the item up, until every overlapping request has got that far.
type heldBody struct {
	content io.Reader
	arrived *sync.WaitGroup
	release <-chan struct{}
	once    sync.Once
}

func (b *heldBody) arrive() {
	b.once.Do(b.arrived.Done)
}

func (b *heldBody) Read(buffer []byte) (int, error) {
	b.arrive()
	<-b.release
	return b.content.Read(buffer)
}

// linkTogether sends one link request per invoice straight to the handler and
// lets them continue only once all are reading their bodies.
func (c *testClient) linkTogether(t *testing.T, itemID int64, invoices ...int64) []int {
	t.Helper()
	handler, err := c.server.application.Handler()
	if err != nil {
		t.Fatal(err)
	}
	var arrived, finished sync.WaitGroup
	release := make(chan struct{})
	statuses := make([]int, len(invoices))
	address, _ := url.Parse(c.server.http.URL)
	arrived.Add(len(invoices))
	finished.Add(len(invoices))
	for index, invoice := range invoices {
		encoded, _ := json.Marshal(map[string]any{"attachmentIds": []int64{invoice}})
		body := &heldBody{content: bytes.NewReader(encoded), arrived: &arrived, release: release}
		request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/items/%d/attachments", testOrigin, itemID), body)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", c.origin)
		request.Header.Set("X-CSRF-Token", c.csrf)
		for _, cookie := range c.client.Jar.Cookies(address) {
			request.AddCookie(cookie)
		}
		go func() {
			defer finished.Done()
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			// A request answered before reading its body still counts as arrived.
			body.arrive()
			statuses[index] = recorder.Code
		}()
	}
	arrived.Wait()
	close(release)
	finished.Wait()
	return statuses
}

func TestOverlappingLinkRequestsKeepEveryReceipt(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	item := client.createItem(map[string]any{"name": "Name only"})
	receipt, warranty := client.uploadInvoice(t, "receipt.pdf"), client.uploadInvoice(t, "warranty.pdf")
	for _, status := range client.linkTogether(t, item.ID, receipt, warranty) {
		if status != http.StatusNoContent {
			t.Fatalf("an overlapping link failed: %d", status)
		}
	}
	documents := func() int {
		var reloaded itemResponse
		client.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d", item.ID), nil).json(t, &reloaded)
		return len(reloaded.Documents)
	}
	if documents() != 2 || purchaseCount(server) != 1 {
		t.Fatalf("overlapping links kept %d of 2 receipts in %d purchases", documents(), purchaseCount(server))
	}

	client.linkTogether(t, item.ID, receipt, warranty)
	if documents() != 2 || purchaseCount(server) != 1 {
		t.Fatalf("repeating the links gave %d receipts in %d purchases", documents(), purchaseCount(server))
	}

	if result := client.do(http.MethodDelete, fmt.Sprintf("/api/v1/items/%d/attachments/%d", item.ID, receipt), nil); result.Status != http.StatusNoContent {
		t.Fatalf("unlink: %d %s", result.Status, result.Body)
	}
	if documents() != 1 {
		t.Fatalf("unlinking one receipt left %d", documents())
	}

	trashed := client.createItem(map[string]any{"name": "Deleted"})
	client.do(http.MethodDelete, fmt.Sprintf("/api/v1/items/%d", trashed.ID), nil)
	late := client.uploadInvoice(t, "late.pdf")
	if result := client.do(http.MethodPost, fmt.Sprintf("/api/v1/items/%d/attachments", trashed.ID), map[string]any{"attachmentIds": []int64{late}}); result.Status != http.StatusNotFound {
		t.Fatalf("linking to a deleted item answered %d", result.Status)
	}
	if purchaseCount(server) != 1 {
		t.Fatal("a refused link left a purchase behind")
	}
}
