package server

import (
	"fmt"
	"net/http"
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
