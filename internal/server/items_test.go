package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
)

type itemResponse struct {
	ID           int64              `json:"id"`
	Name         string             `json:"name"`
	Revision     int64              `json:"revision"`
	PricePaid    *map[string]string `json:"pricePaid"`
	PurchaseDate *map[string]string `json:"purchaseDate"`
	Room         *map[string]any    `json:"room"`
	Tags         []map[string]any   `json:"tags"`
	PurchaseID   int64              `json:"purchaseId"`
	SharedWith   []map[string]any   `json:"sharedWith"`
	Photos       []map[string]any   `json:"photos"`
	Documents    []map[string]any   `json:"documents"`
	PrimaryPhoto int64              `json:"primaryPhotoId"`
	Duplicates   []map[string]any   `json:"possibleDuplicates"`
}

func (c *testClient) createItem(body map[string]any) itemResponse {
	c.server.t.Helper()
	result := c.do(http.MethodPost, "/api/v1/items", body)
	if result.Status != http.StatusCreated {
		c.server.t.Fatalf("create: %d %s", result.Status, result.Body)
	}
	var item itemResponse
	result.json(c.server.t, &item)
	return item
}

func (c *testClient) put(path string, revision int64, body any) response {
	request, _ := http.NewRequest(http.MethodPut, c.server.http.URL+path, strings.NewReader(mustJSON(body)))
	request.Header.Set("Origin", c.origin)
	request.Header.Set("X-CSRF-Token", c.csrf)
	request.Header.Set("If-Match", strconv.Quote(strconv.FormatInt(revision, 10)))
	result, err := c.client.Do(request)
	if err != nil {
		c.server.t.Fatal(err)
	}
	defer result.Body.Close()
	var buffer bytes.Buffer
	buffer.ReadFrom(result.Body)
	return response{Status: result.StatusCode, Body: buffer.Bytes()}
}

func mustJSON(value any) string {
	var buffer bytes.Buffer
	encoder := jsonEncoder(&buffer)
	encoder.Encode(value)
	return buffer.String()
}

func TestNameOnlyCaptureAndIdempotentSave(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()

	item := client.createItem(map[string]any{"name": "Cordless drill"})
	if item.PricePaid != nil || item.PurchaseDate != nil || item.Room != nil {
		t.Fatalf("name-only item gained values: %+v", item)
	}

	request, _ := http.NewRequest(http.MethodPost, server.http.URL+"/api/v1/items", strings.NewReader(`{"name":"Kettle"}`))
	request.Header.Set("Origin", testOrigin)
	request.Header.Set("X-CSRF-Token", client.csrf)
	request.Header.Set("Idempotency-Key", "form-123")
	first, _ := client.client.Do(request.Clone(request.Context()))
	request2, _ := http.NewRequest(http.MethodPost, server.http.URL+"/api/v1/items", strings.NewReader(`{"name":"Kettle"}`))
	request2.Header = request.Header.Clone()
	second, _ := client.client.Do(request2)
	first.Body.Close()
	second.Body.Close()
	var kettles int
	server.application.Store.Database.QueryRow(`SELECT count(*) FROM items WHERE name = 'Kettle'`).Scan(&kettles)
	if kettles != 1 {
		t.Fatalf("double submit created %d items", kettles)
	}

	invalid := client.do(http.MethodPost, "/api/v1/items", map[string]any{"name": "  ", "pricePaid": map[string]string{"amount": "abc", "currency": "GBP"}})
	var problem struct {
		Fields map[string]string `json:"fields"`
	}
	invalid.json(t, &problem)
	if problem.Fields["name"] == "" || problem.Fields["pricePaid"] == "" {
		t.Fatalf("field errors missing: %s", invalid.Body)
	}
}

func TestExactMoneyDatesAndConflicts(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()

	yen := client.createItem(map[string]any{"name": "Rice cooker", "pricePaid": map[string]string{"amount": "12800", "currency": "JPY"}})
	dinar := client.createItem(map[string]any{"name": "Lamp", "pricePaid": map[string]string{"amount": "12.345", "currency": "BHD"},
		"purchaseDate": map[string]string{"value": "2024-03-12"}})
	if (*yen.PricePaid)["amount"] != "12800" || (*dinar.PricePaid)["amount"] != "12.345" {
		t.Fatalf("money changed: %v %v", *yen.PricePaid, *dinar.PricePaid)
	}
	if (*dinar.PurchaseDate)["value"] != "2024-03-12" || (*dinar.PurchaseDate)["precision"] != "date" {
		t.Fatalf("date precision lost: %v", *dinar.PurchaseDate)
	}
	var minor int64
	server.application.Store.Database.QueryRow(`SELECT paid_unit_minor FROM items WHERE id = ?`, dinar.ID).Scan(&minor)
	if minor != 12345 {
		t.Fatalf("stored minor units %d", minor)
	}
	if result := client.do(http.MethodPost, "/api/v1/items", map[string]any{"name": "x", "pricePaid": map[string]string{"amount": "1.5", "currency": "JPY"}}); result.Status != http.StatusBadRequest {
		t.Fatal("fractional yen accepted")
	}

	updated := client.put(fmt.Sprintf("/api/v1/items/%d", dinar.ID), dinar.Revision, map[string]any{"name": "Desk lamp", "purchaseDate": nil})
	if updated.Status != http.StatusOK {
		t.Fatalf("update: %s", updated.Body)
	}
	var after itemResponse
	updated.json(t, &after)
	if after.PurchaseDate != nil || after.PricePaid != nil || after.Revision != dinar.Revision+1 {
		t.Fatalf("cleared values not cleared: %s", updated.Body)
	}
	stale := client.put(fmt.Sprintf("/api/v1/items/%d", dinar.ID), dinar.Revision, map[string]any{"name": "Overwrite"})
	if stale.Status != http.StatusConflict {
		t.Fatalf("stale revision accepted: %d", stale.Status)
	}

	var history []map[string]any
	client.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d/history", dinar.ID), nil).json(t, &history)
	if len(history) != 2 || !contains([]byte(mustJSON(history[0])), "Desk lamp") {
		t.Fatalf("history: %v", history)
	}
}

func TestOrganisationSearchAndTrash(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()

	machine := client.createItem(map[string]any{"name": "Sage Barista Express", "seller": "John Lewis", "room": "Kitchen", "tags": []string{"Coffee", "coffee", "Appliance"}})
	client.createItem(map[string]any{"name": "Toaster", "room": "kitchen"})
	client.createItem(map[string]any{"name": "Headphones"})
	if len(machine.Tags) != 2 {
		t.Fatalf("duplicate tag names not merged: %v", machine.Tags)
	}

	var rooms []map[string]any
	client.do(http.MethodGet, "/api/v1/groups/rooms", nil).json(t, &rooms)
	if len(rooms) != 1 || rooms[0]["itemCount"].(float64) != 2 {
		t.Fatalf("rooms: %v", rooms)
	}

	search := func(query string) int {
		var page struct {
			Total int `json:"total"`
		}
		client.do(http.MethodGet, "/api/v1/items?"+query, nil).json(t, &page)
		return page.Total
	}
	for query, want := range map[string]int{"q=coff": 1, "q=john": 1, "q=kitchen": 2, "q=%22%29%20OR%20*": 0, "unassigned=1": 1, "priceKnown=1": 0} {
		if got := search(query); got != want {
			t.Errorf("search %s = %d want %d", query, got, want)
		}
	}

	roomID := int64(rooms[0]["id"].(float64))
	if result := client.do(http.MethodDelete, fmt.Sprintf("/api/v1/groups/rooms/%d", roomID), nil); result.Status != http.StatusNoContent {
		t.Fatalf("delete room: %s", result.Body)
	}
	if got := search("unassigned=1"); got != 3 {
		t.Fatalf("items lost or not unassigned after deleting room: %d", got)
	}

	if result := client.do(http.MethodDelete, fmt.Sprintf("/api/v1/items/%d", machine.ID), nil); result.Status != http.StatusNoContent {
		t.Fatal("trash failed")
	}
	if search("q=barista") != 0 || search("deleted=1") != 1 {
		t.Fatal("trashed item still searchable or missing from trash")
	}
	client.do(http.MethodPost, fmt.Sprintf("/api/v1/items/%d/restore", machine.ID), nil)
	if search("q=barista") != 1 {
		t.Fatal("restore failed")
	}
	if result := client.do(http.MethodDelete, fmt.Sprintf("/api/v1/trash/%d", machine.ID), nil); result.Status != http.StatusNotFound {
		t.Fatal("purge allowed for an item not in trash")
	}

	again := client.createItem(map[string]any{"name": "sage barista express", "seller": "John Lewis"})
	if len(again.Duplicates) != 1 {
		t.Fatalf("duplicate not flagged: %+v", again)
	}
}

func pngBytes(width int, height int) []byte {
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	canvas.Set(0, 0, color.RGBA{R: 200, A: 255})
	var buffer bytes.Buffer
	png.Encode(&buffer, canvas)
	return buffer.Bytes()
}

// hugePNGHeader claims enormous dimensions without containing the pixels.
func hugePNGHeader() []byte {
	var buffer bytes.Buffer
	buffer.Write([]byte("\x89PNG\r\n\x1a\n"))
	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header[0:], 30000)
	binary.BigEndian.PutUint32(header[4:], 30000)
	header[8], header[9] = 8, 6
	chunk := append([]byte("IHDR"), header...)
	binary.Write(&buffer, binary.BigEndian, uint32(len(header)))
	buffer.Write(chunk)
	binary.Write(&buffer, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return buffer.Bytes()
}

func (c *testClient) upload(kind string, filename string, contentType string, content []byte) response {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	header.Set("Content-Type", contentType)
	part, _ := writer.CreatePart(header)
	part.Write(content)
	writer.Close()
	request, _ := http.NewRequest(http.MethodPost, c.server.http.URL+"/api/v1/attachments?kind="+kind, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", c.origin)
	request.Header.Set("X-CSRF-Token", c.csrf)
	result, err := c.client.Do(request)
	if err != nil {
		c.server.t.Fatal(err)
	}
	defer result.Body.Close()
	var buffer bytes.Buffer
	buffer.ReadFrom(result.Body)
	return response{Status: result.StatusCode, Body: buffer.Bytes(), Header: result.Header}
}

func TestUploadValidationAndAuthorisedDownload(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	pdf := []byte("%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF")

	cases := []struct {
		name, kind, filename, contentType string
		content                           []byte
		wantCode                          string
	}{
		{"svg", "photo", "a.svg", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), "unsupported_file"},
		{"html as png", "photo", "a.png", "image/png", []byte("<html><script>alert(1)</script></html>"), "unsupported_file"},
		{"png named pdf", "invoice", "a.pdf", "application/pdf", pngBytes(4, 4), "file_type_mismatch"},
		{"pdf as photo", "photo", "a.pdf", "application/pdf", pdf, "unsupported_photo"},
		{"declared mismatch", "photo", "a.png", "image/jpeg", pngBytes(4, 4), "file_type_mismatch"},
		{"pixel bomb", "photo", "big.png", "image/png", hugePNGHeader(), "image_too_large"},
		{"oversized", "document", "big.pdf", "application/pdf", append(pdf, make([]byte, 21<<20)...), "file_too_large"},
	}
	for _, testCase := range cases {
		result := client.upload(testCase.kind, testCase.filename, testCase.contentType, testCase.content)
		if result.code() != testCase.wantCode {
			t.Errorf("%s: got %d %s", testCase.name, result.Status, result.Body)
		}
	}
	var attachmentRows int
	server.application.Store.Database.QueryRow(`SELECT count(*) FROM attachments`).Scan(&attachmentRows)
	if attachmentRows != 0 {
		t.Fatalf("failed uploads left %d attachment records", attachmentRows)
	}

	photo := client.upload("photo", "../../etc/passwd.png", "image/png", pngBytes(2000, 1000))
	invoice := client.upload("invoice", "Invoice – Sage.pdf", "application/pdf", pdf)
	var photoBody, invoiceBody struct {
		ID       int64  `json:"id"`
		Filename string `json:"filename"`
	}
	photo.json(t, &photoBody)
	invoice.json(t, &invoiceBody)
	if photoBody.Filename != "passwd.png" {
		t.Fatalf("filename not sanitised: %q", photoBody.Filename)
	}

	first := client.createItem(map[string]any{"name": "Coffee machine", "attachmentIds": []int64{photoBody.ID, invoiceBody.ID}})
	second := client.createItem(map[string]any{"name": "Grinder", "purchaseId": first.PurchaseID})
	if first.PrimaryPhoto != photoBody.ID || len(first.Documents) != 1 {
		t.Fatalf("links: %+v", first)
	}
	var grinder itemResponse
	client.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d", second.ID), nil).json(t, &grinder)
	if len(grinder.Documents) != 1 || len(grinder.SharedWith) != 1 {
		t.Fatalf("shared invoice not visible on second item: %+v", grinder)
	}

	download := client.do(http.MethodGet, fmt.Sprintf("/api/v1/attachments/%d", invoiceBody.ID), nil)
	disposition := download.Header.Get("Content-Disposition")
	if download.Status != http.StatusOK || !strings.HasPrefix(disposition, "attachment") || download.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("download headers: %d %v", download.Status, download.Header)
	}
	preview := client.do(http.MethodGet, fmt.Sprintf("/api/v1/attachments/%d?variant=preview", photoBody.ID), nil)
	decoded, _, err := image.DecodeConfig(bytes.NewReader(preview.Body))
	if err != nil || decoded.Width != 1600 {
		t.Fatalf("preview not resized: %v %+v", err, decoded)
	}
	if result := server.newClient().do(http.MethodGet, fmt.Sprintf("/api/v1/attachments/%d", invoiceBody.ID), nil); result.Status != http.StatusUnauthorized {
		t.Fatalf("anonymous download: %d", result.Status)
	}

	database := server.application.Store.Database
	database.Exec(`INSERT INTO households (id, name, created_at) VALUES (99, 'Other', 'now')`)
	database.Exec(`INSERT INTO attachments (id, household_id, kind, storage_key, filename, media_type, size_bytes, sha256, created_at) VALUES (999, 99, 'invoice', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'x.pdf', 'application/pdf', 1, '', 'now')`)
	if result := client.do(http.MethodGet, "/api/v1/attachments/999", nil); result.Status != http.StatusNotFound {
		t.Fatalf("cross-household download: %d", result.Status)
	}
	if result := client.do(http.MethodPost, "/api/v1/items", map[string]any{"name": "x", "attachmentIds": []int64{999}}); result.Status != http.StatusBadRequest {
		t.Fatalf("linked another household's file: %d", result.Status)
	}

	var documents []map[string]any
	client.do(http.MethodGet, "/api/v1/documents?q=sage", nil).json(t, &documents)
	if len(documents) != 2 {
		t.Fatalf("documents across shared items: %v", documents)
	}
}

func TestRoomDescriptionAndPhoto(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	photo := client.upload("photo", "kitchen.png", "image/png", pngBytes(20, 20))
	var photoBody struct {
		ID int64 `json:"id"`
	}
	photo.json(t, &photoBody)

	created := client.do(http.MethodPost, "/api/v1/groups/rooms", map[string]any{"name": "Kitchen", "description": "Ground floor, by the garden door", "photoId": photoBody.ID})
	if created.Status != http.StatusCreated {
		t.Fatalf("create room: %s", created.Body)
	}
	var rooms []map[string]any
	client.do(http.MethodGet, "/api/v1/groups/rooms", nil).json(t, &rooms)
	if rooms[0]["description"] != "Ground floor, by the garden door" || int64(rooms[0]["photoId"].(float64)) != photoBody.ID {
		t.Fatalf("room details: %v", rooms[0])
	}

	roomID := int64(rooms[0]["id"].(float64))
	if result := client.do(http.MethodPatch, fmt.Sprintf("/api/v1/groups/rooms/%d", roomID), map[string]any{"photoId": 0}); result.Status != http.StatusOK {
		t.Fatalf("remove photo: %s", result.Body)
	}
	rooms = nil
	client.do(http.MethodGet, "/api/v1/groups/rooms", nil).json(t, &rooms)
	if _, hasPhoto := rooms[0]["photoId"]; hasPhoto || rooms[0]["name"] != "Kitchen" {
		t.Fatalf("photo not removed or name changed: %v", rooms[0])
	}
	if result := client.do(http.MethodPost, "/api/v1/groups/tags", map[string]any{"name": "Audio", "description": "x"}); result.code() != "rooms_only" {
		t.Fatalf("tag accepted a description: %s", result.Body)
	}
	if result := client.do(http.MethodPatch, fmt.Sprintf("/api/v1/groups/rooms/%d", roomID), map[string]any{"photoId": 999999}); result.code() != "invalid_photo" {
		t.Fatalf("unknown photo accepted: %s", result.Body)
	}

	client.do(http.MethodPatch, fmt.Sprintf("/api/v1/groups/rooms/%d", roomID), map[string]any{"photoId": photoBody.ID})
	server.application.Store.Database.Exec(`UPDATE attachments SET created_at = '2000-01-01T00:00:00Z'`)
	server.application.Items.PurgeAbandonedAttachments(context.Background())
	var remaining int
	server.application.Store.Database.QueryRow(`SELECT count(*) FROM attachments WHERE id = ?`, photoBody.ID).Scan(&remaining)
	if remaining != 1 {
		t.Fatal("a room photo was removed as an abandoned upload")
	}
}

func TestConditionAndWarranty(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	covered := client.createItem(map[string]any{"name": "Fridge", "condition": "good", "warrantyUntil": "2099-12-31"})
	client.createItem(map[string]any{"name": "Old kettle", "condition": "needs_repair", "warrantyUntil": "2001-01-01"})
	client.createItem(map[string]any{"name": "Unknown lamp"})

	var fridge map[string]any
	client.do(http.MethodGet, fmt.Sprintf("/api/v1/items/%d", covered.ID), nil).json(t, &fridge)
	if fridge["condition"] != "good" || fridge["warrantyUntil"] != "2099-12-31" {
		t.Fatalf("not stored: %v", fridge)
	}
	count := func(query string) int {
		var page struct {
			Total int `json:"total"`
		}
		client.do(http.MethodGet, "/api/v1/items?"+query, nil).json(t, &page)
		return page.Total
	}
	if count("inWarranty=1") != 1 || count("condition=needs_repair") != 1 || count("condition=new") != 0 {
		t.Fatalf("filters wrong: warranty %d repair %d", count("inWarranty=1"), count("condition=needs_repair"))
	}
	for _, bad := range []map[string]any{{"name": "x", "condition": "shiny"}, {"name": "x", "warrantyUntil": "2025-13-40"}} {
		if result := client.do(http.MethodPost, "/api/v1/items", bad); result.Status != http.StatusBadRequest {
			t.Errorf("accepted %v", bad)
		}
	}
	if _, err := server.application.Store.Database.Exec(`UPDATE items SET condition = 'shiny'`); err == nil {
		t.Fatal("database accepted an unknown condition")
	}
}

func TestRoomCostsAreExactAndPerCurrency(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()
	client.createItem(map[string]any{"name": "Chair", "room": "Kitchen", "quantity": 4, "pricePaid": map[string]string{"amount": "19.99", "currency": "GBP"}})
	client.createItem(map[string]any{"name": "Kettle", "room": "Kitchen", "pricePaid": map[string]string{"amount": "0.03", "currency": "GBP"}})
	client.createItem(map[string]any{"name": "Coffee grinder", "room": "Kitchen", "pricePaid": map[string]string{"amount": "80", "currency": "EUR"}})
	client.createItem(map[string]any{"name": "Old bowl", "room": "Kitchen"})
	client.createItem(map[string]any{"name": "Lamp", "room": "Bedroom", "pricePaid": map[string]string{"amount": "25", "currency": "GBP"}})
	client.createItem(map[string]any{"name": "Charger", "pricePaid": map[string]string{"amount": "9.50", "currency": "GBP"}})
	trashed := client.createItem(map[string]any{"name": "Broken TV", "room": "Bedroom", "pricePaid": map[string]string{"amount": "500", "currency": "GBP"}})
	client.do(http.MethodDelete, fmt.Sprintf("/api/v1/items/%d", trashed.ID), nil)

	var costs struct {
		Rooms map[string]struct {
			Totals        map[string]string `json:"totals"`
			UnpricedItems int               `json:"unpricedItems"`
		} `json:"rooms"`
		AllRooms struct {
			Totals map[string]string `json:"totals"`
		} `json:"allRooms"`
		Unassigned struct {
			Totals map[string]string `json:"totals"`
		} `json:"unassigned"`
	}
	client.do(http.MethodGet, "/api/v1/items/room-costs", nil).json(t, &costs)
	var rooms []map[string]any
	client.do(http.MethodGet, "/api/v1/groups/rooms", nil).json(t, &rooms)
	identifiers := map[string]string{}
	for _, room := range rooms {
		identifiers[room["name"].(string)] = fmt.Sprint(room["id"])
	}
	kitchen := costs.Rooms[identifiers["Kitchen"]]
	if kitchen.Totals["GBP"] != "79.99" || kitchen.Totals["EUR"] != "80.00" || kitchen.UnpricedItems != 1 {
		t.Fatalf("kitchen: %+v", kitchen)
	}
	if costs.Rooms[identifiers["Bedroom"]].Totals["GBP"] != "25.00" {
		t.Fatalf("bedroom counted a deleted item: %+v", costs.Rooms[identifiers["Bedroom"]])
	}
	if costs.AllRooms.Totals["GBP"] != "104.99" || costs.AllRooms.Totals["EUR"] != "80.00" || costs.Unassigned.Totals["GBP"] != "9.50" {
		t.Fatalf("totals: %+v %+v", costs.AllRooms, costs.Unassigned)
	}
}
