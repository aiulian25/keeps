package server

import (
	"fmt"
	"net/http"
	"testing"
)

type purchaseFacts struct {
	Revision       int64             `json:"revision"`
	Seller         string            `json:"seller"`
	OrderReference string            `json:"orderReference"`
	PurchaseDate   map[string]string `json:"purchaseDate"`
	PurchaseID     int64             `json:"purchaseId"`
}

func (c *testClient) facts(t *testing.T, itemID int64) purchaseFacts {
	t.Helper()
	var facts purchaseFacts
	c.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d", itemID), nil).json(t, &facts)
	return facts
}

func TestJoiningAPurchaseKeepsItsFacts(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	original := client.createItem(map[string]any{"name": "Sofa", "seller": "Shop", "orderReference": "ORDER-123",
		"purchaseDate": map[string]string{"value": "2024-03-12", "precision": "date"}})

	sibling := client.createItem(map[string]any{"name": "Footstool", "purchaseId": original.PurchaseID})
	for _, itemID := range []int64{original.ID, sibling.ID} {
		facts := client.facts(t, itemID)
		if facts.Seller != "Shop" || facts.OrderReference != "ORDER-123" || facts.PurchaseDate["value"] != "2024-03-12" || facts.PurchaseDate["precision"] != "date" {
			t.Fatalf("item %d lost purchase facts after a sibling joined: %+v", itemID, facts)
		}
	}

	client.createItem(map[string]any{"name": "Cushion", "purchaseId": original.PurchaseID, "seller": "Shop Ltd"})
	if facts := client.facts(t, original.ID); facts.Seller != "Shop Ltd" || facts.OrderReference != "ORDER-123" {
		t.Fatalf("a stated value should apply and blanks should not clear: %+v", facts)
	}

	current := client.facts(t, original.ID)
	if result := client.put(fmt.Sprintf("/api/v1/items/%d", original.ID), current.Revision, map[string]any{"name": "Sofa", "seller": "Shop Ltd", "orderReference": ""}); result.Status != http.StatusOK {
		t.Fatalf("edit: %d %s", result.Status, result.Body)
	}
	if facts := client.facts(t, sibling.ID); facts.OrderReference != "" {
		t.Fatalf("an intentional edit of the shared purchase did not clear it: %+v", facts)
	}

	if result := client.do(http.MethodPost, "/api/v1/items", map[string]any{"name": "Stranger", "purchaseId": 999999}); result.Status != http.StatusNotFound {
		t.Fatalf("an unknown purchase was accepted: %d", result.Status)
	}
}

func TestStaleSiblingFormCannotRestoreOldSharedFacts(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	first := client.createItem(map[string]any{"name": "Sofa", "seller": "Shop", "orderReference": "ORDER-1"})
	second := client.createItem(map[string]any{"name": "Footstool", "purchaseId": first.PurchaseID})
	staleSecond := client.facts(t, second.ID)

	current := client.facts(t, first.ID)
	if result := client.put(fmt.Sprintf("/api/v1/items/%d", first.ID), current.Revision, map[string]any{"name": "Sofa", "notes": "only notes", "seller": "Shop", "orderReference": "ORDER-1"}); result.Status != http.StatusOK {
		t.Fatalf("no-op shared edit: %d", result.Status)
	}
	if client.facts(t, second.ID).Revision != staleSecond.Revision {
		t.Fatal("a save that left shared facts alone invalidated the sibling")
	}

	current = client.facts(t, first.ID)
	if result := client.put(fmt.Sprintf("/api/v1/items/%d", first.ID), current.Revision, map[string]any{"name": "Sofa", "seller": "New shop", "orderReference": "ORDER-1"}); result.Status != http.StatusOK {
		t.Fatalf("shared edit: %d", result.Status)
	}
	stale := client.put(fmt.Sprintf("/api/v1/items/%d", second.ID), staleSecond.Revision, map[string]any{"name": "Footstool", "notes": "draft", "seller": "Shop", "orderReference": "ORDER-1"})
	if stale.Status != http.StatusConflict {
		t.Fatalf("a stale sibling form was accepted: %d %s", stale.Status, stale.Body)
	}
	if facts := client.facts(t, first.ID); facts.Seller != "New shop" {
		t.Fatalf("the stale form restored the old seller: %+v", facts)
	}
	var notes string
	server.application.Store.Database.QueryRow(`SELECT notes FROM items WHERE id = ?`, second.ID).Scan(&notes)
	if notes != "" {
		t.Fatal("a rejected save partly changed the sibling")
	}

	reloaded := client.facts(t, second.ID)
	if retry := client.put(fmt.Sprintf("/api/v1/items/%d", second.ID), reloaded.Revision, map[string]any{"name": "Footstool", "notes": "draft", "seller": reloaded.Seller, "orderReference": reloaded.OrderReference}); retry.Status != http.StatusOK {
		t.Fatalf("a deliberate retry after reloading failed: %d", retry.Status)
	}
	if facts := client.facts(t, first.ID); facts.Seller != "New shop" {
		t.Fatalf("retry changed shared facts: %+v", facts)
	}
}
