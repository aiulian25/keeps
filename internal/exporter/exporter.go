// Package exporter produces portable copies of a household's data: a CSV for
// spreadsheets and a zip archive with JSON, original files and checksums.
// Neither contains credentials, sessions or instance keys.
package exporter

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aiulian25/keeps/internal/blob"
	"github.com/aiulian25/keeps/internal/items"
)

const (
	exportFormatVersion = 1
	exportPageSize      = 200
	formulaPrefixes     = "=+-@\t\r"
	readmeText          = `Keeps export

items.json        Every item with all of its details, and under "groups" every
                  room (with its description and photo), category, tag and
                  collection, including empty ones; under "bookmarks" the named
                  pages of exported PDFs; under "presets" your saved starting
                  details for new items. Versioned JSON format.
files/            The original photos, invoices and documents, including room
                  photos. File names start with the attachment ID used in
                  items.json. A file shared by several items is included once.
checksums.sha256  SHA-256 checksums; verify with: sha256sum -c checksums.sha256

Not included: items in Recently deleted, edit history, uploads never added to
an item or room, and passwords, sign-in methods or the instance key.
To move a complete instance, use a backup instead.
`
)

// ErrFileMissing means a photo or document the items refer to is not in storage.
var ErrFileMissing = errors.New("file missing from storage")

// ErrFileDamaged means a stored original no longer matches the size and
// SHA-256 recorded when it was uploaded.
var ErrFileDamaged = errors.New("file damaged in storage")

var csvHeader = []string{"id", "name", "description", "quantity", "brand", "model", "serial_number", "units", "condition", "warranty_until", "measurements", "room", "category", "tags", "collections",
	"seller", "order_reference", "price_paid", "currency", "purchase_date", "purchase_date_precision", "notes", "source_url", "created_at", "updated_at"}

// SafeCell neutralises values a spreadsheet would treat as a formula.
func SafeCell(value string) string {
	if value != "" && strings.ContainsRune(formulaPrefixes, rune(value[0])) {
		return "'" + value
	}
	return value
}

func allItems(ctx context.Context, service *items.Service, householdID int64, visit func(items.Item) error) error {
	cursor := ""
	for {
		page, err := service.List(ctx, householdID, items.ListQuery{Sort: "name", Cursor: cursor, Limit: exportPageSize})
		if err != nil {
			return err
		}
		for _, summary := range page.Items {
			item, err := service.Get(ctx, householdID, summary.ID)
			if err != nil {
				return err
			}
			if err := visit(item); err != nil {
				return err
			}
		}
		if page.NextCursor == "" {
			return nil
		}
		cursor = page.NextCursor
	}
}

func names(references []items.Reference) string {
	list := make([]string, 0, len(references))
	for _, reference := range references {
		list = append(list, reference.Name)
	}
	return strings.Join(list, "; ")
}

func optionalName(reference *items.Reference) string {
	if reference == nil {
		return ""
	}
	return reference.Name
}

func WriteCSV(ctx context.Context, service *items.Service, householdID int64, destination io.Writer) error {
	writer := csv.NewWriter(destination)
	if err := writer.Write(csvHeader); err != nil {
		return err
	}
	err := allItems(ctx, service, householdID, func(item items.Item) error {
		price, currency, date, precision := "", "", "", ""
		if item.PricePaid != nil {
			price, currency = item.PricePaid.Amount, item.PricePaid.Currency
		}
		if item.PurchaseDate != nil {
			date, precision = item.PurchaseDate.Value, item.PurchaseDate.Precision
		}
		row := []string{fmt.Sprint(item.ID), item.Name, item.Description, fmt.Sprint(item.Quantity), item.Brand, item.Model, item.SerialNumber, items.UnitsSummary(item.Units),
			item.Condition, item.WarrantyUntil, item.Measurements.Summary(), optionalName(item.Room), optionalName(item.Category), names(item.Tags), names(item.Collections), item.Seller, item.OrderReference,
			price, currency, date, precision, item.Notes, item.SourceURL, item.CreatedAt, item.UpdatedAt}
		for index := range row {
			row[index] = SafeCell(row[index])
		}
		return writer.Write(row)
	})
	if err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

type archiveDocument struct {
	Format     int          `json:"format"`
	ExportedAt string       `json:"exportedAt"`
	Items      []items.Item `json:"items"`
	Groups     groupsExport `json:"groups"`
	// Bookmarks are named pages in exported PDFs, by attachment ID.
	Bookmarks []bookmarkExport `json:"bookmarks"`
	// Presets are the household's reusable starting details for new items.
	Presets []presetExport `json:"presets"`
}

type presetExport struct {
	Name        string          `json:"name"`
	Room        string          `json:"room"`
	Category    string          `json:"category"`
	Tags        json.RawMessage `json:"tags"`
	Collections json.RawMessage `json:"collections"`
	Condition   string          `json:"condition"`
	Notes       string          `json:"notes"`
}

type bookmarkExport struct {
	AttachmentID int64  `json:"attachmentId"`
	Page         int    `json:"page"`
	Label        string `json:"label"`
}

// groupsExport lists every grouping, so rooms and empty groups survive the
// export, not only the ones items point at.
type groupsExport struct {
	Rooms       []roomExport      `json:"rooms"`
	Categories  []items.Reference `json:"categories"`
	Tags        []items.Reference `json:"tags"`
	Collections []items.Reference `json:"collections"`
}

type roomExport struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PhotoID     int64  `json:"photoId,omitempty"`
	PhotoFile   string `json:"photoFile,omitempty"`
}

// WriteArchive streams a zip of items.json, original files and checksums.
func WriteArchive(ctx context.Context, service *items.Service, database *sql.DB, blobs *blob.Store, householdID int64, destination io.Writer) error {
	archive := zip.NewWriter(destination)
	var checksums strings.Builder
	document := archiveDocument{Format: exportFormatVersion, ExportedAt: time.Now().UTC().Format(time.RFC3339), Items: []items.Item{}}
	written := map[int64]bool{}

	include := func(attachmentID int64, filename string, owner string) (string, error) {
		name := fmt.Sprintf("files/%d-%s", attachmentID, filename)
		if written[attachmentID] {
			return name, nil
		}
		written[attachmentID] = true
		sum, err := copyAttachment(ctx, database, blobs, archive, householdID, attachmentID, name)
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%w: %s (%s)", ErrFileMissing, filename, owner)
		}
		if errors.Is(err, ErrFileDamaged) {
			return "", fmt.Errorf("%w: %s (%s)", ErrFileDamaged, filename, owner)
		}
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&checksums, "%s  %s\n", sum, name)
		return name, nil
	}

	err := allItems(ctx, service, householdID, func(item items.Item) error {
		document.Items = append(document.Items, item)
		for _, attachment := range append(append([]items.Attachment{}, item.Photos...), item.Documents...) {
			if _, err := include(attachment.ID, attachment.Filename, item.Name); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	document.Groups, err = readGroups(ctx, database, householdID)
	if err != nil {
		return err
	}
	document.Bookmarks, err = readBookmarks(ctx, database, householdID, written)
	if err != nil {
		return err
	}
	document.Presets, err = readPresets(ctx, database, householdID)
	if err != nil {
		return err
	}
	for index, room := range document.Groups.Rooms {
		if room.PhotoID == 0 {
			continue
		}
		var filename string
		if err := database.QueryRowContext(ctx, `SELECT filename FROM attachments WHERE id = ? AND household_id = ?`, room.PhotoID, householdID).Scan(&filename); err != nil {
			return err
		}
		document.Groups.Rooms[index].PhotoFile, err = include(room.PhotoID, filename, room.Name)
		if err != nil {
			return err
		}
	}

	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	for name, content := range map[string][]byte{"items.json": encoded, "README.txt": []byte(readmeText)} {
		sum, err := writeEntry(archive, name, strings.NewReader(string(content)))
		if err != nil {
			return err
		}
		fmt.Fprintf(&checksums, "%s  %s\n", sum, name)
	}
	if _, err := writeEntry(archive, "checksums.sha256", strings.NewReader(checksums.String())); err != nil {
		return err
	}
	return archive.Close()
}

func readPresets(ctx context.Context, database *sql.DB, householdID int64) ([]presetExport, error) {
	rows, err := database.QueryContext(ctx, `SELECT name, room, category, tags_json, collections_json, condition, notes FROM capture_presets WHERE household_id = ? ORDER BY name`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	presets := []presetExport{}
	for rows.Next() {
		var preset presetExport
		var tags, collections string
		if err := rows.Scan(&preset.Name, &preset.Room, &preset.Category, &tags, &collections, &preset.Condition, &preset.Notes); err != nil {
			return nil, err
		}
		preset.Tags, preset.Collections = json.RawMessage(tags), json.RawMessage(collections)
		presets = append(presets, preset)
	}
	return presets, rows.Err()
}

// readBookmarks lists the bookmarks of documents that are in the archive.
func readBookmarks(ctx context.Context, database *sql.DB, householdID int64, included map[int64]bool) ([]bookmarkExport, error) {
	rows, err := database.QueryContext(ctx, `SELECT attachment_id, page, label FROM document_bookmarks WHERE household_id = ? ORDER BY attachment_id, page, id`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bookmarks := []bookmarkExport{}
	for rows.Next() {
		var bookmark bookmarkExport
		if err := rows.Scan(&bookmark.AttachmentID, &bookmark.Page, &bookmark.Label); err != nil {
			return nil, err
		}
		if included[bookmark.AttachmentID] {
			bookmarks = append(bookmarks, bookmark)
		}
	}
	return bookmarks, rows.Err()
}

func readGroups(ctx context.Context, database *sql.DB, householdID int64) (groupsExport, error) {
	groups := groupsExport{Rooms: []roomExport{}}
	rows, err := database.QueryContext(ctx, `SELECT id, name, description, coalesce(photo_id, 0) FROM rooms WHERE household_id = ? ORDER BY name`, householdID)
	if err != nil {
		return groups, err
	}
	for rows.Next() {
		var room roomExport
		if err := rows.Scan(&room.ID, &room.Name, &room.Description, &room.PhotoID); err != nil {
			rows.Close()
			return groups, err
		}
		groups.Rooms = append(groups.Rooms, room)
	}
	rows.Close()
	for table, destination := range map[string]*[]items.Reference{"categories": &groups.Categories, "tags": &groups.Tags, "collections": &groups.Collections} {
		*destination, err = readNames(ctx, database, table, householdID)
		if err != nil {
			return groups, err
		}
	}
	return groups, nil
}

// readNames lists one simple grouping table; table is one of a fixed set.
func readNames(ctx context.Context, database *sql.DB, table string, householdID int64) ([]items.Reference, error) {
	rows, err := database.QueryContext(ctx, `SELECT id, name FROM `+table+` WHERE household_id = ? ORDER BY name`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	references := []items.Reference{}
	for rows.Next() {
		var reference items.Reference
		if err := rows.Scan(&reference.ID, &reference.Name); err != nil {
			return nil, err
		}
		references = append(references, reference)
	}
	return references, rows.Err()
}

func copyAttachment(ctx context.Context, database *sql.DB, blobs *blob.Store, archive *zip.Writer, householdID int64, attachmentID int64, name string) (string, error) {
	var key, recordedSHA string
	if err := database.QueryRowContext(ctx, `SELECT storage_key, sha256 FROM attachments WHERE id = ? AND household_id = ?`, attachmentID, householdID).Scan(&key, &recordedSHA); err != nil {
		return "", err
	}
	file, err := blobs.Open(key)
	if err != nil {
		return "", fmt.Errorf("file for attachment %d is missing: %w", attachmentID, err)
	}
	defer file.Close()
	sum, err := writeEntry(archive, name, file)
	if err == nil && sum != recordedSHA {
		return "", ErrFileDamaged
	}
	return sum, err
}

func writeEntry(archive *zip.Writer, name string, content io.Reader) (string, error) {
	entry, err := archive.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()})
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(entry, hash), content); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
