package server

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

type presetResponse struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Room    string   `json:"room"`
	Tags    []string `json:"tags"`
	Missing []string `json:"missing"`
}

func TestCapturePresetsHoldOnlyOrganisingDetails(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()

	if result := client.do(http.MethodPost, "/api/v1/presets", map[string]any{"name": "Empty"}); result.code() != "invalid_preset" {
		t.Fatalf("an empty preset was saved: %s", result.Body)
	}
	if result := client.do(http.MethodPost, "/api/v1/presets", map[string]any{"name": "Kit", "room": "Garage", "serialNumber": "SN1"}); result.Status != http.StatusBadRequest {
		t.Fatalf("a preset accepted a serial number: %d %s", result.Status, result.Body)
	}
	created := client.do(http.MethodPost, "/api/v1/presets", map[string]any{"name": "Garage tools", "room": "Garage", "category": "Tools", "tags": []string{"Power", "power", " DIY "}, "condition": "good", "notes": "Check the battery."})
	if created.Status != http.StatusCreated {
		t.Fatalf("save: %d %s", created.Status, created.Body)
	}
	var preset presetResponse
	created.json(t, &preset)
	if fmt.Sprint(preset.Tags) != "[Power DIY]" {
		t.Fatalf("tags not cleaned: %v", preset.Tags)
	}
	if result := client.do(http.MethodPost, "/api/v1/presets", map[string]any{"name": "garage TOOLS", "room": "Shed"}); result.code() != "preset_exists" {
		t.Fatalf("duplicate name: %s", result.Body)
	}

	var presets []presetResponse
	client.do(http.MethodGet, "/api/v1/presets", nil).json(t, &presets)
	if len(presets) != 1 || len(presets[0].Missing) != 4 {
		t.Fatalf("groups that don't exist yet should be listed as missing: %+v", presets)
	}
	client.createItem(map[string]any{"name": "Drill", "room": "Garage", "category": "Tools", "tags": []string{"Power", "DIY"}})
	client.do(http.MethodGet, "/api/v1/presets", nil).json(t, &presets)
	if len(presets[0].Missing) != 0 {
		t.Fatalf("existing groups reported missing: %+v", presets[0].Missing)
	}

	export := client.fetchDownload("/api/v1/exports", map[string]string{"format": "archive"})
	if !strings.Contains(string(zipEntry(t, export.Body, "items.json")), `"name": "Garage tools"`) {
		t.Fatal("presets are missing from the export")
	}
	if result := client.do(http.MethodDelete, fmt.Sprintf("/api/v1/presets/%d", preset.ID), nil); result.Status != http.StatusNoContent {
		t.Fatalf("delete: %d", result.Status)
	}
	if result := client.do(http.MethodDelete, fmt.Sprintf("/api/v1/presets/%d", preset.ID), nil); result.code() != "preset_not_found" {
		t.Fatalf("deleted twice: %s", result.Body)
	}
}
