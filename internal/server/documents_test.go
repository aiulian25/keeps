package server

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestEveryDocumentIsReachableThroughPages(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	database := server.application.Store.Database
	var householdID int64
	database.QueryRow(`SELECT household_id FROM users LIMIT 1`).Scan(&householdID)
	const sameMoment = "2026-01-01T00:00:00Z"
	transaction, _ := database.Begin()
	transaction.Exec(`INSERT INTO purchases (id, household_id, created_at) VALUES (1, ?, ?)`, householdID, sameMoment)
	for index := 1; index <= 260; index++ {
		transaction.Exec(`INSERT INTO items (id, household_id, name, purchase_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			index, householdID, fmt.Sprintf("Item %03d", index), map[bool]any{true: 1, false: nil}[index <= 2], sameMoment, sameMoment)
		transaction.Exec(`INSERT INTO attachments (id, household_id, kind, storage_key, filename, media_type, size_bytes, sha256, created_at) VALUES (?, ?, 'document', ?, ?, 'application/pdf', 1, 'x', ?)`,
			index, householdID, fmt.Sprintf("%032x", index), fmt.Sprintf("manual-%03d.pdf", index), sameMoment)
		transaction.Exec(`INSERT INTO item_attachments (item_id, attachment_id) VALUES (?, ?)`, index, index)
		extra := 1000 + index
		transaction.Exec(`INSERT INTO attachments (id, household_id, kind, storage_key, filename, media_type, size_bytes, sha256, created_at) VALUES (?, ?, 'document', ?, ?, 'application/pdf', 1, 'x', ?)`,
			extra, householdID, fmt.Sprintf("%032x", extra), fmt.Sprintf("warranty-%03d.pdf", index), sameMoment)
		transaction.Exec(`INSERT INTO item_attachments (item_id, attachment_id) VALUES (?, ?)`, index, extra)
	}
	transaction.Exec(`INSERT INTO attachments (id, household_id, kind, storage_key, filename, media_type, size_bytes, sha256, created_at) VALUES (5000, ?, 'invoice', ?, 'shared-receipt.pdf', 'application/pdf', 1, 'x', ?)`, householdID, fmt.Sprintf("%032x", 5000), sameMoment)
	transaction.Exec(`INSERT INTO purchase_attachments (purchase_id, attachment_id) VALUES (1, 5000)`)
	if err := transaction.Commit(); err != nil {
		t.Fatal(err)
	}
	const associations = 260*2 + 2

	var legacy []map[string]any
	client.do(http.MethodGet, "/api/v1/documents?q=", nil).json(t, &legacy)
	if len(legacy) != 500 {
		t.Fatalf("the unpaged v1 response changed: %d rows", len(legacy))
	}

	seen := map[string]bool{}
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 20 {
			t.Fatal("paging never ended")
		}
		var page struct {
			Documents []struct {
				ID     int64 `json:"id"`
				ItemID int64 `json:"itemId"`
			} `json:"documents"`
			Total      int    `json:"total"`
			NextCursor string `json:"nextCursor"`
		}
		client.do(http.MethodGet, "/api/v1/documents?paged=1&cursor="+url.QueryEscape(cursor), nil).json(t, &page)
		if page.Total != associations {
			t.Fatalf("total %d, want %d", page.Total, associations)
		}
		for _, entry := range page.Documents {
			key := fmt.Sprintf("%d/%d", entry.ID, entry.ItemID)
			if seen[key] {
				t.Fatalf("%s appeared on two pages", key)
			}
			seen[key] = true
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(seen) != associations {
		t.Fatalf("reached %d of %d associations", len(seen), associations)
	}

	var narrowed struct {
		Documents []map[string]any `json:"documents"`
		Total     int              `json:"total"`
	}
	client.do(http.MethodGet, "/api/v1/documents?paged=1&q=shared-receipt", nil).json(t, &narrowed)
	if narrowed.Total != 2 || len(narrowed.Documents) != 2 {
		t.Fatalf("a shared receipt should be listed under both items: %+v", narrowed)
	}
	if bad := client.do(http.MethodGet, "/api/v1/documents?paged=1&cursor=nonsense!", nil); bad.code() != "invalid_cursor" {
		t.Fatalf("a broken cursor: %d %s", bad.Status, bad.Body)
	}
}

func TestFullArchiveKeepsRoomsAndEmptyGroups(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	var photo struct {
		ID int64 `json:"id"`
	}
	client.upload("photo", "loft.png", "image/png", pngBytes(12, 12)).json(t, &photo)
	client.do(http.MethodPost, "/api/v1/groups/rooms", map[string]any{"name": "Loft", "description": "Boxes and the old desk", "photoId": photo.ID})
	client.do(http.MethodPost, "/api/v1/groups/tags", map[string]any{"name": "Unused tag"})
	receipt := client.uploadInvoice(t, "shared.pdf")
	first := client.createItem(map[string]any{"name": "Chair", "attachmentIds": []int64{receipt}})
	client.createItem(map[string]any{"name": "Table", "purchaseId": first.PurchaseID})

	archive := client.fetchDownload("/api/v1/exports", map[string]string{"format": "archive"})
	reader, err := zip.NewReader(bytes.NewReader(archive.Body), int64(len(archive.Body)))
	if err != nil {
		t.Fatal(err)
	}
	contents := map[string][]byte{}
	for _, file := range reader.File {
		opened, _ := file.Open()
		contents[file.Name], _ = io.ReadAll(opened)
		opened.Close()
	}
	var document struct {
		Groups struct {
			Rooms []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				PhotoFile   string `json:"photoFile"`
			} `json:"rooms"`
			Tags []struct {
				Name string `json:"name"`
			} `json:"tags"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(contents["items.json"], &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Groups.Rooms) != 1 || document.Groups.Rooms[0].Description != "Boxes and the old desk" || document.Groups.Rooms[0].PhotoFile == "" {
		t.Fatalf("the empty room was not exported with its details: %+v", document.Groups.Rooms)
	}
	roomPhoto := contents[document.Groups.Rooms[0].PhotoFile]
	if len(roomPhoto) == 0 || !bytes.Equal(roomPhoto[:8], pngBytes(12, 12)[:8]) {
		t.Fatal("the room photo original is missing from files/")
	}
	if len(document.Groups.Tags) != 1 || document.Groups.Tags[0].Name != "Unused tag" {
		t.Fatalf("an empty tag was lost: %+v", document.Groups.Tags)
	}
	sharedCopies := 0
	for name := range contents {
		if strings.HasSuffix(name, "-shared.pdf") {
			sharedCopies++
		}
	}
	if sharedCopies != 1 {
		t.Fatalf("a receipt shared by two items was included %d times", sharedCopies)
	}
	sum := sha256.Sum256(roomPhoto)
	if !bytes.Contains(contents["checksums.sha256"], []byte(hex.EncodeToString(sum[:])+"  "+document.Groups.Rooms[0].PhotoFile)) {
		t.Fatal("the room photo has no matching checksum")
	}
}
