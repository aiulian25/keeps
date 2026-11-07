package server

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDocumentBookmarksAreNamedPagesThatTravelWithExports(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	manual := client.uploadInvoice(t, "manual.pdf")
	client.createItem(map[string]any{"name": "Boiler", "attachmentIds": []int64{manual}})
	var photo struct {
		ID int64 `json:"id"`
	}
	client.upload("photo", "boiler.png", "image/png", pngBytes(8, 8)).json(t, &photo)

	path := fmt.Sprintf("/api/v1/attachments/%d/bookmarks", manual)
	for name, body := range map[string]map[string]any{
		"page 0":   {"page": 0, "label": "Front"},
		"no label": {"page": 3, "label": "  "},
	} {
		if result := client.do(http.MethodPost, path, body); result.code() != "invalid_bookmark" {
			t.Errorf("%s accepted: %d %s", name, result.Status, result.Body)
		}
	}
	if result := client.do(http.MethodPost, fmt.Sprintf("/api/v1/attachments/%d/bookmarks", photo.ID), map[string]any{"page": 1, "label": "Photo"}); result.Status != http.StatusNotFound {
		t.Fatalf("a photo was bookmarked: %d", result.Status)
	}
	client.do(http.MethodPost, path, map[string]any{"page": 12, "label": "Warranty exclusions"})
	created := client.do(http.MethodPost, path, map[string]any{"page": 3, "label": " Wiring diagram "})
	var bookmark struct {
		ID int64 `json:"id"`
	}
	created.json(t, &bookmark)

	var listed []struct {
		Page  int    `json:"page"`
		Label string `json:"label"`
	}
	client.do(http.MethodGet, path, nil).json(t, &listed)
	if len(listed) != 2 || listed[0].Page != 3 || listed[0].Label != "Wiring diagram" {
		t.Fatalf("bookmarks not listed by page: %+v", listed)
	}

	archive := client.fetchDownload("/api/v1/exports", map[string]string{"format": "archive"})
	reader, _ := zip.NewReader(bytes.NewReader(archive.Body), int64(len(archive.Body)))
	for _, file := range reader.File {
		if file.Name != "items.json" {
			continue
		}
		opened, _ := file.Open()
		content, _ := io.ReadAll(opened)
		opened.Close()
		if !strings.Contains(string(content), `"label": "Warranty exclusions"`) {
			t.Fatal("bookmarks are missing from the export")
		}
	}

	if result := client.do(http.MethodDelete, fmt.Sprintf("%s/%d", path, bookmark.ID), nil); result.Status != http.StatusNoContent {
		t.Fatalf("remove: %d", result.Status)
	}
	client.do(http.MethodGet, path, nil).json(t, &listed)
	if len(listed) != 1 {
		t.Fatalf("removed bookmark still listed: %+v", listed)
	}
}

func zipEntry(t *testing.T, archive []byte, name string) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range reader.File {
		if file.Name == name {
			opened, _ := file.Open()
			content, _ := io.ReadAll(opened)
			opened.Close()
			return content
		}
	}
	return nil
}
