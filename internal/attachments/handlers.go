package attachments

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"math"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/aiulian25/keeps/internal/auth"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/items"
	"github.com/aiulian25/keeps/internal/store"
)

const (
	variantOriginal  = "original"
	variantPreview   = "preview"
	variantThumbnail = "thumbnail"
	previewCache     = "private, max-age=86400, immutable"
	uploadOverhead   = 1 << 20
)

func (s *Service) Routes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	handle := func(pattern string, handler httpapi.Handler) {
		mux.Handle(pattern, protect(handler))
	}

	handle("POST /api/v1/attachments", func(writer http.ResponseWriter, request *http.Request) error {
		request.Body = http.MaxBytesReader(writer, request.Body, config.AttachmentMaximumBytes+uploadOverhead)
		attachment, err := s.Upload(request.Context(), auth.PrincipalFromRequest(request).HouseholdID, request.URL.Query().Get("kind"), request)
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return ErrTooLarge
		}
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusCreated, attachment)
		return nil
	})

	handle("GET /api/v1/attachments/{id}", s.handleDownload)
	s.bookmarkRoutes(handle)

	handle("PATCH /api/v1/attachments/{id}", func(writer http.ResponseWriter, request *http.Request) error {
		identifier, err := httpapi.PathID(request)
		if err != nil {
			return err
		}
		var body struct {
			Filename string `json:"filename"`
		}
		if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &body); err != nil {
			return err
		}
		householdID := auth.PrincipalFromRequest(request).HouseholdID
		if _, err := s.load(request.Context(), householdID, identifier); err != nil {
			return err
		}
		_, err = s.store.Database.ExecContext(request.Context(), `UPDATE attachments SET filename = ? WHERE id = ? AND household_id = ?`, CleanFilename(body.Filename), identifier, householdID)
		if err != nil {
			return err
		}
		writer.WriteHeader(http.StatusNoContent)
		return nil
	})

	handle("POST /api/v1/items/{id}/attachments", s.handleLink)
	handle("DELETE /api/v1/items/{id}/attachments/{attachmentId}", s.handleUnlink)
	handle("PUT /api/v1/items/{id}/primary-photo", s.handlePrimaryPhoto)
	handle("GET /api/v1/documents", s.handleDocuments)
}

// handleDownload serves a file only after the household check. Originals and
// PDFs are always downloads (never rendered inline); previews are images.
func (s *Service) handleDownload(writer http.ResponseWriter, request *http.Request) error {
	identifier, err := httpapi.PathID(request)
	if err != nil {
		return err
	}
	stored, err := s.load(request.Context(), auth.PrincipalFromRequest(request).HouseholdID, identifier)
	if err != nil {
		return err
	}
	variant := request.URL.Query().Get("variant")
	key, mediaType, inline := stored.storageKey, stored.MediaType, false
	switch {
	case variant == variantPreview && stored.previewKey != "":
		key, mediaType, inline = stored.previewKey, mediaJPEG, true
	case variant == variantThumbnail && stored.thumbnailKey != "":
		key, mediaType, inline = stored.thumbnailKey, mediaJPEG, true
	case variant != "" && variant != variantOriginal:
		return httpapi.ErrNotFound
	}

	file, err := s.blobs.Open(key)
	if err != nil {
		return httpapi.NewError(http.StatusGone, "file_unavailable", "This file is missing from storage. Restore it from a backup or upload it again.")
	}
	defer file.Close()

	headers := writer.Header()
	headers.Set("Content-Type", mediaType)
	headers.Set("X-Content-Type-Options", "nosniff")
	headers.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	if inline {
		headers.Set("Cache-Control", previewCache)
		headers.Set("Content-Disposition", "inline")
	} else {
		headers.Set("Cache-Control", "private, no-store")
		headers.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": stored.Filename}))
		headers.Set("Content-Length", strconv.FormatInt(stored.SizeBytes, 10))
	}
	_, err = io.Copy(writer, file)
	return err
}

func (s *Service) itemFor(request *http.Request) (int64, int64, error) {
	identifier, err := httpapi.PathID(request)
	if err != nil {
		return 0, 0, err
	}
	householdID := auth.PrincipalFromRequest(request).HouseholdID
	var found int
	err = s.store.Database.QueryRowContext(request.Context(), `SELECT 1 FROM items WHERE id = ? AND household_id = ? AND deleted_at IS NULL`, identifier, householdID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, httpapi.ErrNotFound
	}
	return identifier, householdID, err
}

// currentPurchase reads the item's purchase inside the write transaction, so
// overlapping requests build on each other's purchase instead of each making
// one and the later replacing the earlier.
func currentPurchase(ctx context.Context, transaction *sql.Tx, itemID int64, householdID int64) (int64, error) {
	var purchase sql.NullInt64
	err := transaction.QueryRowContext(ctx, `SELECT purchase_id FROM items WHERE id = ? AND household_id = ? AND deleted_at IS NULL`, itemID, householdID).Scan(&purchase)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, httpapi.ErrNotFound
	}
	return purchase.Int64, err
}

func (s *Service) handleLink(writer http.ResponseWriter, request *http.Request) error {
	itemID, householdID, err := s.itemFor(request)
	if err != nil {
		return err
	}
	var body struct {
		AttachmentIDs []int64 `json:"attachmentIds"`
	}
	if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &body); err != nil {
		return err
	}
	err = s.store.Transaction(request.Context(), func(transaction *sql.Tx) error {
		purchaseID, err := currentPurchase(request.Context(), transaction, itemID, householdID)
		if err != nil {
			return err
		}
		for _, attachmentID := range body.AttachmentIDs {
			var kind string
			if err := transaction.QueryRowContext(request.Context(), `SELECT kind FROM attachments WHERE id = ? AND household_id = ?`, attachmentID, householdID).Scan(&kind); err != nil {
				return httpapi.ErrNotFound
			}
			purchaseID, err = items.LinkAttachment(request.Context(), transaction, householdID, itemID, purchaseID, attachmentID, kind)
			if err != nil {
				return err
			}
		}
		_, err = transaction.ExecContext(request.Context(), `UPDATE items SET revision = revision + 1, updated_at = ? WHERE id = ?`, store.Now(), itemID)
		return err
	})
	if err != nil {
		return err
	}
	writer.WriteHeader(http.StatusNoContent)
	return nil
}

// handleUnlink removes a file from an item. An invoice on a shared purchase
// is removed from that purchase, so every item sharing it loses it; the UI
// warns before this.
func (s *Service) handleUnlink(writer http.ResponseWriter, request *http.Request) error {
	itemID, householdID, err := s.itemFor(request)
	if err != nil {
		return err
	}
	attachmentID, err := strconv.ParseInt(request.PathValue("attachmentId"), 10, 64)
	if err != nil {
		return httpapi.ErrNotFound
	}
	if _, err := s.load(request.Context(), householdID, attachmentID); err != nil {
		return err
	}
	err = s.store.Transaction(request.Context(), func(transaction *sql.Tx) error {
		purchaseID, err := currentPurchase(request.Context(), transaction, itemID, householdID)
		if err != nil {
			return err
		}
		if _, err := transaction.ExecContext(request.Context(), `DELETE FROM item_attachments WHERE item_id = ? AND attachment_id = ?`, itemID, attachmentID); err != nil {
			return err
		}
		if _, err := transaction.ExecContext(request.Context(), `DELETE FROM purchase_attachments WHERE purchase_id = ? AND attachment_id = ?`, purchaseID, attachmentID); err != nil {
			return err
		}
		_, err = transaction.ExecContext(request.Context(), `UPDATE items SET primary_photo_id = (SELECT x.attachment_id FROM item_attachments x JOIN attachments a ON a.id = x.attachment_id
				WHERE x.item_id = items.id AND a.kind = 'photo' ORDER BY x.position LIMIT 1)
			WHERE id = ? AND (primary_photo_id = ? OR primary_photo_id IS NULL)`, itemID, attachmentID)
		if err != nil {
			return err
		}
		_, err = transaction.ExecContext(request.Context(), `UPDATE items SET revision = revision + 1, updated_at = ? WHERE id = ?`, store.Now(), itemID)
		return err
	})
	if err != nil {
		return err
	}
	writer.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Service) handlePrimaryPhoto(writer http.ResponseWriter, request *http.Request) error {
	itemID, _, err := s.itemFor(request)
	if err != nil {
		return err
	}
	var body struct {
		AttachmentID int64 `json:"attachmentId"`
	}
	if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &body); err != nil {
		return err
	}
	result, err := s.store.Database.ExecContext(request.Context(), `UPDATE items SET primary_photo_id = ?, revision = revision + 1, updated_at = ?
		WHERE id = ? AND EXISTS (SELECT 1 FROM item_attachments x JOIN attachments a ON a.id = x.attachment_id WHERE x.item_id = ? AND x.attachment_id = ? AND a.kind = 'photo')`,
		body.AttachmentID, store.Now(), itemID, itemID, body.AttachmentID)
	if err != nil {
		return err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return httpapi.ErrNotFound
	}
	writer.WriteHeader(http.StatusNoContent)
	return nil
}

type DocumentEntry struct {
	Attachment
	ItemID   int64  `json:"itemId"`
	ItemName string `json:"itemName"`
}

// handleDocuments lists invoices and documents across all items.
// documentsQuery lists every document association: one row per item a file
// belongs to, so a receipt shared by a purchase appears under each item.
const documentsQuery = `SELECT a.id AS attachment_id, a.kind, a.filename, a.media_type, a.size_bytes, a.preview_key IS NOT NULL AS has_preview,
		a.created_at, i.id AS item_id, i.name AS item_name
	FROM attachments a
	JOIN (SELECT attachment_id, item_id FROM item_attachments
		UNION SELECT pa.attachment_id, i2.id FROM purchase_attachments pa JOIN items i2 ON i2.purchase_id = pa.purchase_id) link ON link.attachment_id = a.id
	JOIN items i ON i.id = link.item_id
	WHERE a.household_id = ? AND a.kind != 'photo' AND i.deleted_at IS NULL AND (a.filename LIKE ? OR i.name LIKE ?)`

const legacyDocumentsLimit = 500

// documentsPage is the paged form of GET /documents (?paged=1).
type documentsPage struct {
	Documents  []DocumentEntry `json:"documents"`
	Total      int             `json:"total"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

var errInvalidCursor = httpapi.NewError(http.StatusBadRequest, "invalid_cursor", "That page of documents is no longer valid. Load the list again.")

// handleDocuments returns the newest documents first. Without ?paged=1 it
// keeps the original v1 contract: a plain array of up to 500 rows. With it,
// it returns pages ordered by (created_at, attachment, item), so rows with
// the same time are neither skipped nor repeated between pages.
func (s *Service) handleDocuments(writer http.ResponseWriter, request *http.Request) error {
	householdID := auth.PrincipalFromRequest(request).HouseholdID
	query := request.URL.Query()
	search := "%" + query.Get("q") + "%"
	if query.Get("paged") != "1" {
		documents, err := s.queryDocuments(request.Context(), `SELECT * FROM (`+documentsQuery+`) ORDER BY created_at DESC, attachment_id DESC, item_id DESC LIMIT ?`,
			householdID, search, search, legacyDocumentsLimit)
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusOK, documents)
		return nil
	}

	limit := config.DocumentsPageSize
	after := []any{"\uffff", int64(math.MaxInt64), int64(math.MaxInt64)}
	if cursor := query.Get("cursor"); cursor != "" {
		parsed, err := parseDocumentsCursor(cursor)
		if err != nil {
			return errInvalidCursor
		}
		after = parsed
	}
	arguments := append([]any{householdID, search, search}, after...)
	arguments = append(arguments, limit+1)
	documents, err := s.queryDocuments(request.Context(), `SELECT * FROM (`+documentsQuery+`)
		WHERE (created_at, attachment_id, item_id) < (?, ?, ?)
		ORDER BY created_at DESC, attachment_id DESC, item_id DESC LIMIT ?`, arguments...)
	if err != nil {
		return err
	}
	page := documentsPage{Documents: documents}
	if len(documents) > limit {
		page.Documents = documents[:limit]
		last := page.Documents[limit-1]
		page.NextCursor = documentsCursor(last)
	}
	if err := s.store.Database.QueryRowContext(request.Context(), `SELECT count(*) FROM (`+documentsQuery+`)`, householdID, search, search).Scan(&page.Total); err != nil {
		return err
	}
	httpapi.WriteJSON(writer, http.StatusOK, page)
	return nil
}

func (s *Service) queryDocuments(ctx context.Context, statement string, arguments ...any) ([]DocumentEntry, error) {
	rows, err := s.store.Database.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	documents := []DocumentEntry{}
	for rows.Next() {
		var entry DocumentEntry
		if err := rows.Scan(&entry.ID, &entry.Kind, &entry.Filename, &entry.MediaType, &entry.SizeBytes, &entry.HasPreview, &entry.CreatedAt, &entry.ItemID, &entry.ItemName); err != nil {
			return nil, err
		}
		documents = append(documents, entry)
	}
	return documents, rows.Err()
}

const cursorSeparator = "|"

func documentsCursor(last DocumentEntry) string {
	raw := strings.Join([]string{last.CreatedAt, strconv.FormatInt(last.ID, 10), strconv.FormatInt(last.ItemID, 10)}, cursorSeparator)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func parseDocumentsCursor(cursor string) ([]any, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(string(raw), cursorSeparator)
	if len(parts) != 3 {
		return nil, errInvalidCursor
	}
	attachmentID, attachmentErr := strconv.ParseInt(parts[1], 10, 64)
	itemID, itemErr := strconv.ParseInt(parts[2], 10, 64)
	if attachmentErr != nil || itemErr != nil {
		return nil, errInvalidCursor
	}
	return []any{parts[0], attachmentID, itemID}, nil
}
