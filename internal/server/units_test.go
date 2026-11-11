package server

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

// identifiedUnitsCap is how many units one item can have identified.
const identifiedUnitsCap = 100

type unitResponse struct {
	ID           int64  `json:"id"`
	Label        string `json:"label"`
	SerialNumber string `json:"serialNumber"`
}

type itemUnits struct {
	Quantity     int                `json:"quantity"`
	SerialNumber string             `json:"serialNumber"`
	PricePaid    *map[string]string `json:"pricePaid"`
	Revision     int64              `json:"revision"`
	Units        []unitResponse     `json:"units"`
}

func (c *testClient) itemUnits(itemID int64) itemUnits {
	c.server.t.Helper()
	var item itemUnits
	c.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d", itemID), nil).json(c.server.t, &item)
	return item
}

func (c *testClient) addUnit(itemID int64, body map[string]string) unitResponse {
	c.server.t.Helper()
	result := c.do(http.MethodPost, fmt.Sprintf("/api/v1/items/%d/units", itemID), body)
	if result.Status != http.StatusCreated {
		c.server.t.Fatalf("add unit: %d %s", result.Status, result.Body)
	}
	var unit unitResponse
	result.json(c.server.t, &unit)
	return unit
}

func (c *testClient) searchFinds(text string, itemID int64) bool {
	c.server.t.Helper()
	var found struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	c.do(http.MethodGet, "/api/v1/items?q="+url.QueryEscape(text), nil).json(c.server.t, &found)
	return len(found.Items) == 1 && found.Items[0].ID == itemID
}

func TestUnitsAreIdentifiedWithinTheQuantity(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	speakers := client.createItem(map[string]any{"name": "Bookshelf speakers", "quantity": 2, "serialNumber": "PAIR-100", "pricePaid": map[string]string{"amount": "149.50", "currency": "GBP"}})
	other := client.createItem(map[string]any{"name": "Smoke alarm"})
	unitsPath := fmt.Sprintf("/api/v1/items/%d/units", speakers.ID)

	for name, body := range map[string]map[string]string{
		"empty":       {"label": " ", "serialNumber": ""},
		"long serial": {"serialNumber": string(bytes.Repeat([]byte("9"), 201))},
		"long label":  {"label": string(bytes.Repeat([]byte("l"), 201))},
	} {
		if result := client.do(http.MethodPost, unitsPath, body); result.code() != "invalid_unit" {
			t.Errorf("%s accepted: %d %s", name, result.Status, result.Body)
		}
	}
	left := client.addUnit(speakers.ID, map[string]string{"label": " Left speaker ", "serialNumber": " ZX-L1 "})
	right := client.addUnit(speakers.ID, map[string]string{"serialNumber": "AB-R2"})
	if result := client.do(http.MethodPost, unitsPath, map[string]string{"label": "A third"}); result.code() != "all_units_identified" {
		t.Fatalf("a third unit of two was identified: %d %s", result.Status, result.Body)
	}

	item := client.itemUnits(speakers.ID)
	want := []unitResponse{{left.ID, "Left speaker", "ZX-L1"}, {right.ID, "", "AB-R2"}}
	if fmt.Sprint(item.Units) != fmt.Sprint(want) {
		t.Fatalf("units not kept in order as typed: %+v", item.Units)
	}
	if item.Quantity != 2 || item.SerialNumber != "PAIR-100" || item.PricePaid == nil || (*item.PricePaid)["amount"] != "149.50" {
		t.Fatalf("identifying units changed the item's own details: %+v", item)
	}

	if !client.searchFinds("AB-R2", speakers.ID) {
		t.Fatal("a unit's serial number doesn't find its item")
	}

	itemPath := fmt.Sprintf("/api/v1/items/%d", speakers.ID)
	lowered := client.put(itemPath, item.Revision, map[string]any{"name": "Bookshelf speakers", "quantity": 1})
	if lowered.code() != "quantity_below_units" || !contains(lowered.Body, `"quantity"`) {
		t.Fatalf("the quantity fell below the identified units: %d %s", lowered.Status, lowered.Body)
	}
	if after := client.itemUnits(speakers.ID); after.Quantity != 2 || len(after.Units) != 2 {
		t.Fatalf("a refused quantity change still changed the item: %+v", after)
	}

	if result := client.do(http.MethodPut, fmt.Sprintf("/api/v1/items/%d/units/%d", other.ID, left.ID), map[string]string{"label": "Moved"}); result.code() != "unit_not_found" {
		t.Fatalf("a unit was changed through another item: %d %s", result.Status, result.Body)
	}
	renamed := client.do(http.MethodPut, fmt.Sprintf("%s/%d", unitsPath, right.ID), map[string]string{"label": "Right speaker", "serialNumber": "AB-R2"})
	if renamed.Status != http.StatusOK {
		t.Fatalf("rename: %d %s", renamed.Status, renamed.Body)
	}
	if !client.searchFinds("Right", speakers.ID) {
		t.Fatal("a unit's new label doesn't find its item")
	}

	spreadsheet := client.fetchDownload("/api/v1/exports", map[string]string{"format": "csv"})
	rows, _ := csv.NewReader(bytes.NewReader(spreadsheet.Body)).ReadAll()
	column := -1
	for index, heading := range rows[0] {
		if heading == "units" {
			column = index
		}
	}
	summary := ""
	for _, row := range rows[1:] {
		if row[1] == "Bookshelf speakers" && column >= 0 {
			summary = row[column]
		}
	}
	if summary != "Left speaker: ZX-L1; Right speaker: AB-R2" {
		t.Fatalf("the CSV lost the units: %q", summary)
	}

	if result := client.do(http.MethodDelete, fmt.Sprintf("%s/%d", unitsPath, right.ID), nil); result.Status != http.StatusNoContent {
		t.Fatalf("remove: %d %s", result.Status, result.Body)
	}
	if client.searchFinds("AB-R2", speakers.ID) {
		t.Fatal("a removed unit's serial number still finds the item")
	}
	item = client.itemUnits(speakers.ID)
	if result := client.put(itemPath, item.Revision, map[string]any{"name": "Bookshelf speakers", "quantity": 1}); result.Status != http.StatusOK {
		t.Fatalf("lowering the quantity after removing the extra unit: %d %s", result.Status, result.Body)
	}
}

func TestUnitsOfATrashedItemAreKeptAsTheyWere(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	alarms := client.createItem(map[string]any{"name": "Smoke alarms", "quantity": 3})
	unitsPath := fmt.Sprintf("/api/v1/items/%d/units", alarms.ID)
	hallway := client.addUnit(alarms.ID, map[string]string{"label": "Hallway", "serialNumber": "SA-1"})
	unitPath := fmt.Sprintf("%s/%d", unitsPath, hallway.ID)
	itemPath := fmt.Sprintf("/api/v1/items/%d", alarms.ID)

	if result := client.do(http.MethodDelete, itemPath, nil); result.Status != http.StatusNoContent {
		t.Fatalf("trash: %d %s", result.Status, result.Body)
	}
	if result := client.do(http.MethodPost, unitsPath, map[string]string{"label": "Kitchen"}); result.Status != http.StatusNotFound {
		t.Fatalf("a unit was added to a trashed item: %d %s", result.Status, result.Body)
	}
	if result := client.do(http.MethodPut, unitPath, map[string]string{"label": "Changed"}); result.code() != "unit_not_found" {
		t.Fatalf("a trashed item's unit was edited: %d %s", result.Status, result.Body)
	}
	if result := client.do(http.MethodDelete, unitPath, nil); result.code() != "unit_not_found" {
		t.Fatalf("a trashed item's unit was removed: %d %s", result.Status, result.Body)
	}

	if result := client.do(http.MethodPost, itemPath+"/restore", nil); result.Status >= http.StatusBadRequest {
		t.Fatalf("restore: %d %s", result.Status, result.Body)
	}
	if item := client.itemUnits(alarms.ID); len(item.Units) != 1 || item.Units[0] != hallway {
		t.Fatalf("the restored item came back with different units: %+v", item.Units)
	}
}

func TestIdentifiedUnitsAreCapped(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	brackets := client.createItem(map[string]any{"name": "Shelf brackets", "quantity": 500})
	for index := range identifiedUnitsCap {
		client.addUnit(brackets.ID, map[string]string{"serialNumber": fmt.Sprintf("BR-%03d", index)})
	}
	if result := client.do(http.MethodPost, fmt.Sprintf("/api/v1/items/%d/units", brackets.ID), map[string]string{"serialNumber": "BR-100"}); result.code() != "too_many_units" {
		t.Fatalf("a 101st unit was stored: %d %s", result.Status, result.Body)
	}
}
