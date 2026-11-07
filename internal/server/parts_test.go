package server

import (
	"fmt"
	"net/http"
	"testing"
)

type itemParts struct {
	Parts []struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		PartNumber string `json:"partNumber"`
		SourceURL  string `json:"sourceUrl"`
	} `json:"parts"`
}

func TestCompatiblePartsAreKeptPerItem(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	vacuum := client.createItem(map[string]any{"name": "Vacuum"})
	partsPath := fmt.Sprintf("/api/v1/items/%d/parts", vacuum.ID)

	for name, body := range map[string]map[string]string{
		"no name":      {"partNumber": "VB-12"},
		"script url":   {"name": "Bags", "sourceUrl": "javascript:alert(1)"},
		"relative url": {"name": "Bags", "sourceUrl": "/bags"},
	} {
		if result := client.do(http.MethodPost, partsPath, body); result.code() != "invalid_part" {
			t.Errorf("%s accepted: %d %s", name, result.Status, result.Body)
		}
	}
	created := client.do(http.MethodPost, partsPath, map[string]string{"name": " Vacuum bags ", "partNumber": "VB-12", "note": "Type G only", "sourceUrl": "https://maker.example/bags"})
	if created.Status != http.StatusCreated {
		t.Fatalf("add: %d %s", created.Status, created.Body)
	}
	var part struct {
		ID int64 `json:"id"`
	}
	created.json(t, &part)

	var item itemParts
	client.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d", vacuum.ID), nil).json(t, &item)
	if len(item.Parts) != 1 || item.Parts[0].Name != "Vacuum bags" || item.Parts[0].PartNumber != "VB-12" {
		t.Fatalf("part not on the item: %+v", item.Parts)
	}

	other := client.createItem(map[string]any{"name": "Kettle"})
	if result := client.do(http.MethodPut, fmt.Sprintf("/api/v1/items/%d/parts/%d", other.ID, part.ID), map[string]string{"name": "Moved"}); result.code() != "part_not_found" {
		t.Fatalf("a part was edited through another item: %d", result.Status)
	}
	if result := client.do(http.MethodPost, "/api/v1/items/999999/parts", map[string]string{"name": "Ghost"}); result.Status != http.StatusNotFound {
		t.Fatalf("a part was added to a missing item: %d", result.Status)
	}
	if result := client.do(http.MethodPut, fmt.Sprintf("%s/%d", partsPath, part.ID), map[string]string{"name": "Vacuum bags", "partNumber": "VB-14"}); result.Status != http.StatusOK {
		t.Fatalf("edit: %d %s", result.Status, result.Body)
	}
	if result := client.do(http.MethodDelete, fmt.Sprintf("%s/%d", partsPath, part.ID), nil); result.Status != http.StatusNoContent {
		t.Fatalf("remove: %d", result.Status)
	}
	client.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d", vacuum.ID), nil).json(t, &item)
	if len(item.Parts) != 0 {
		t.Fatalf("removed part still listed: %+v", item.Parts)
	}
}
