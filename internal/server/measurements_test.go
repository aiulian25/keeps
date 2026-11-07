package server

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

type measured struct {
	Revision     int64             `json:"revision"`
	Measurements map[string]string `json:"measurements"`
}

func TestMeasurementsKeepTheirUnitsAndAreValidated(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()

	for name, measurements := range map[string]map[string]string{
		"no size unit":   {"width": "80"},
		"unknown unit":   {"width": "80", "lengthUnit": "furlong"},
		"not a number":   {"width": "80cm", "lengthUnit": "cm"},
		"zero":           {"weight": "0", "weightUnit": "kg"},
		"no weight unit": {"weight": "12"},
	} {
		result := client.do(http.MethodPost, "/api/v1/items", map[string]any{"name": "Wardrobe", "measurements": measurements})
		var body struct {
			Fields map[string]string `json:"fields"`
		}
		result.json(t, &body)
		if result.Status != http.StatusBadRequest || body.Fields["measurements"] == "" {
			t.Errorf("%s was accepted: %d %s", name, result.Status, result.Body)
		}
	}

	created := client.createItem(map[string]any{"name": "Wardrobe", "notes": "", "measurements": map[string]string{
		"width": " 80 ", "height": "201.5", "lengthUnit": "cm", "weight": "64", "weightUnit": "kg", "fitNote": "Needs an 85 cm doorway",
	}})
	var saved measured
	client.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d", created.ID), nil).json(t, &saved)
	want := map[string]string{"width": "80", "height": "201.5", "depth": "", "lengthUnit": "cm", "weight": "64", "weightUnit": "kg", "fitNote": "Needs an 85 cm doorway"}
	if fmt.Sprint(saved.Measurements) != fmt.Sprint(want) {
		t.Fatalf("measurements not kept exactly: %v", saved.Measurements)
	}

	spreadsheet := client.fetchDownload("/api/v1/exports", map[string]string{"format": "csv"})
	rows, _ := csv.NewReader(bytes.NewReader(spreadsheet.Body)).ReadAll()
	column := -1
	for index, heading := range rows[0] {
		if heading == "measurements" {
			column = index
		}
	}
	if column < 0 || rows[1][column] != "W 80 × H 201.5 cm; 64 kg; Needs an 85 cm doorway" {
		t.Fatalf("the CSV lost the units: %v", rows)
	}

	if result := client.put(fmt.Sprintf("/api/v1/items/%d", created.ID), saved.Revision, map[string]any{"name": "Wardrobe", "measurements": map[string]string{"fitNote": " "}}); result.Status != http.StatusOK {
		t.Fatalf("clearing: %d %s", result.Status, result.Body)
	}
	var cleared measured
	client.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d", created.ID), nil).json(t, &cleared)
	if cleared.Measurements != nil {
		t.Fatalf("blank measurements were kept: %v", cleared.Measurements)
	}
	history := client.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d/history", created.ID), nil)
	if !strings.Contains(string(history.Body), "measurements") {
		t.Fatal("removing measurements was not recorded in history")
	}
}
