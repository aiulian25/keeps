package attachments

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aiulian25/keeps/internal/auth"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/store"
)

// Bookmark names one page of a PDF. The page is checked against the file's
// page count by the viewer; the server only bounds it, because it doesn't
// parse PDFs.
type Bookmark struct {
	ID    int64  `json:"id"`
	Page  int    `json:"page"`
	Label string `json:"label"`
}

const (
	bookmarkLabelMaximum      = 120
	bookmarkPageMaximum       = 100000
	bookmarksPerDocumentLimit = 200
)

var errBookmarkNotFound = httpapi.NewError(http.StatusNotFound, "bookmark_not_found", "That bookmark is no longer there.")

func validateBookmark(input Bookmark) (Bookmark, error) {
	bookmark := Bookmark{Page: input.Page, Label: strings.TrimSpace(input.Label)}
	fields := map[string]string{}
	if bookmark.Label == "" || utf8.RuneCountInString(bookmark.Label) > bookmarkLabelMaximum {
		fields["label"] = "Give the page a short name, up to 120 characters."
	}
	if bookmark.Page < 1 || bookmark.Page > bookmarkPageMaximum {
		fields["page"] = "Choose a page in this document."
	}
	if len(fields) > 0 {
		return Bookmark{}, &httpapi.Error{Status: http.StatusBadRequest, Code: "invalid_bookmark", Message: "Check the bookmark.", Fields: fields}
	}
	return bookmark, nil
}

// ownedDocument checks the attachment is a document of this household.
func ownedDocument(ctx context.Context, database interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, householdID int64, attachmentID int64) error {
	var found int
	err := database.QueryRowContext(ctx, `SELECT 1 FROM attachments WHERE id = ? AND household_id = ? AND kind != 'photo'`, attachmentID, householdID).Scan(&found)
	if err != nil {
		return httpapi.ErrNotFound
	}
	return nil
}

func (s *Service) loadBookmarks(ctx context.Context, attachmentID int64) ([]Bookmark, error) {
	rows, err := s.store.Database.QueryContext(ctx, `SELECT id, page, label FROM document_bookmarks WHERE attachment_id = ? ORDER BY page, id`, attachmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bookmarks := []Bookmark{}
	for rows.Next() {
		var bookmark Bookmark
		if err := rows.Scan(&bookmark.ID, &bookmark.Page, &bookmark.Label); err != nil {
			return nil, err
		}
		bookmarks = append(bookmarks, bookmark)
	}
	return bookmarks, rows.Err()
}

func (s *Service) bookmarkRoutes(handle func(pattern string, handler httpapi.Handler)) {
	handle("GET /api/v1/attachments/{id}/bookmarks", func(writer http.ResponseWriter, request *http.Request) error {
		attachmentID, err := httpapi.PathID(request)
		if err != nil {
			return err
		}
		if err := ownedDocument(request.Context(), s.store.Database, auth.PrincipalFromRequest(request).HouseholdID, attachmentID); err != nil {
			return err
		}
		bookmarks, err := s.loadBookmarks(request.Context(), attachmentID)
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusOK, bookmarks)
		return nil
	})

	handle("POST /api/v1/attachments/{id}/bookmarks", func(writer http.ResponseWriter, request *http.Request) error {
		attachmentID, err := httpapi.PathID(request)
		if err != nil {
			return err
		}
		var input Bookmark
		if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &input); err != nil {
			return err
		}
		bookmark, err := validateBookmark(input)
		if err != nil {
			return err
		}
		householdID := auth.PrincipalFromRequest(request).HouseholdID
		err = s.store.Transaction(request.Context(), func(transaction *sql.Tx) error {
			if err := ownedDocument(request.Context(), transaction, householdID, attachmentID); err != nil {
				return err
			}
			var count int
			transaction.QueryRowContext(request.Context(), `SELECT count(*) FROM document_bookmarks WHERE attachment_id = ?`, attachmentID).Scan(&count)
			if count >= bookmarksPerDocumentLimit {
				return httpapi.NewError(http.StatusBadRequest, "too_many_bookmarks", "A document can have up to "+strconv.Itoa(bookmarksPerDocumentLimit)+" bookmarks.")
			}
			result, err := transaction.ExecContext(request.Context(), `INSERT INTO document_bookmarks (household_id, attachment_id, page, label, created_at) VALUES (?, ?, ?, ?, ?)`,
				householdID, attachmentID, bookmark.Page, bookmark.Label, store.Now())
			if err != nil {
				return err
			}
			bookmark.ID, err = result.LastInsertId()
			return err
		})
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusCreated, bookmark)
		return nil
	})

	handle("DELETE /api/v1/attachments/{id}/bookmarks/{bookmarkId}", func(writer http.ResponseWriter, request *http.Request) error {
		attachmentID, err := httpapi.PathID(request)
		if err != nil {
			return err
		}
		bookmarkID, err := strconv.ParseInt(request.PathValue("bookmarkId"), 10, 64)
		if err != nil {
			return errBookmarkNotFound
		}
		result, err := s.store.Database.ExecContext(request.Context(), `DELETE FROM document_bookmarks WHERE id = ? AND attachment_id = ? AND household_id = ?`,
			bookmarkID, attachmentID, auth.PrincipalFromRequest(request).HouseholdID)
		if err != nil {
			return err
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return errBookmarkNotFound
		}
		writer.WriteHeader(http.StatusNoContent)
		return nil
	})
}
