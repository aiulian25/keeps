package items

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"strconv"

	"github.com/aiulian25/keeps/internal/auth"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/store"
)

// Part is a consumable or replacement part the owner recorded as fitting an
// item: a bag, filter, cartridge or battery. Compatibility is the owner's
// note, not something Keeps checks.
type Part struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	PartNumber string `json:"partNumber"`
	Note       string `json:"note"`
	SourceURL  string `json:"sourceUrl"`
}

const (
	partNoteMaximumLength = 2000
	partURLMaximumLength  = 2000
	partsPerItemMaximum   = 100
)

var errPartNotFound = httpapi.NewError(http.StatusNotFound, "part_not_found", "That part is no longer recorded for this item, or the item was moved to Recently deleted. Open the item again to see it as it is now.")

// partOfActiveItem limits a part change to parts whose item is in the same
// household and not in the trash, so an older tab can't change what a
// restored item brings back.
const partOfActiveItem = `EXISTS (SELECT 1 FROM items i WHERE i.id = item_parts.item_id AND i.household_id = item_parts.household_id AND i.deleted_at IS NULL)`

func validatePart(input Part) (Part, error) {
	part := Part{Name: clean(input.Name), PartNumber: clean(input.PartNumber), Note: clean(input.Note), SourceURL: clean(input.SourceURL)}
	fields := map[string]string{}
	if part.Name == "" {
		fields["name"] = "Enter what the part is, like Vacuum bags."
	}
	if tooLong(part.Name, config.ItemShortTextMaximum) {
		fields["name"] = "Use up to 200 characters."
	}
	if tooLong(part.PartNumber, config.ItemShortTextMaximum) {
		fields["partNumber"] = "Use up to 200 characters."
	}
	if tooLong(part.Note, partNoteMaximumLength) {
		fields["note"] = "Keep the note under 2000 characters."
	}
	if part.SourceURL != "" {
		parsed, err := url.Parse(part.SourceURL)
		validURL := err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
		if !validURL || tooLong(part.SourceURL, partURLMaximumLength) {
			fields["sourceUrl"] = "Enter a full web address starting with https://"
		}
	}
	if len(fields) > 0 {
		return Part{}, &httpapi.Error{Status: http.StatusBadRequest, Code: "invalid_part", Message: "Check the highlighted fields.", Fields: fields}
	}
	return part, nil
}

func loadParts(ctx context.Context, database querier, itemID int64) ([]Part, error) {
	rows, err := database.QueryContext(ctx, `SELECT id, name, part_number, note, source_url FROM item_parts WHERE item_id = ? ORDER BY name, id`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	parts := []Part{}
	for rows.Next() {
		var part Part
		if err := rows.Scan(&part.ID, &part.Name, &part.PartNumber, &part.Note, &part.SourceURL); err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	return parts, rows.Err()
}

// ownedItem checks the item belongs to the household and is not in the trash.
func ownedItem(ctx context.Context, transaction *sql.Tx, householdID int64, itemID int64) error {
	var found int
	err := transaction.QueryRowContext(ctx, `SELECT 1 FROM items WHERE id = ? AND household_id = ? AND deleted_at IS NULL`, itemID, householdID).Scan(&found)
	if err != nil {
		return ErrNotFound
	}
	return nil
}

func (s *Service) partRoutes(handle func(pattern string, handler httpapi.Handler)) {
	decode := func(writer http.ResponseWriter, request *http.Request) (int64, Part, error) {
		itemID, err := httpapi.PathID(request)
		if err != nil {
			return 0, Part{}, err
		}
		var input Part
		if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &input); err != nil {
			return 0, Part{}, err
		}
		part, err := validatePart(input)
		return itemID, part, err
	}

	handle("POST /api/v1/items/{id}/parts", func(writer http.ResponseWriter, request *http.Request) error {
		itemID, part, err := decode(writer, request)
		if err != nil {
			return err
		}
		householdID := auth.PrincipalFromRequest(request).HouseholdID
		err = s.store.Transaction(request.Context(), func(transaction *sql.Tx) error {
			if err := ownedItem(request.Context(), transaction, householdID, itemID); err != nil {
				return err
			}
			var count int
			transaction.QueryRowContext(request.Context(), `SELECT count(*) FROM item_parts WHERE item_id = ?`, itemID).Scan(&count)
			if count >= partsPerItemMaximum {
				return httpapi.NewError(http.StatusBadRequest, "too_many_parts", "An item can have up to "+strconv.Itoa(partsPerItemMaximum)+" parts.")
			}
			now := store.Now()
			result, err := transaction.ExecContext(request.Context(), `INSERT INTO item_parts (household_id, item_id, name, part_number, note, source_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				householdID, itemID, part.Name, part.PartNumber, part.Note, part.SourceURL, now, now)
			if err != nil {
				return err
			}
			part.ID, err = result.LastInsertId()
			return err
		})
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusCreated, part)
		return nil
	})

	handle("PUT /api/v1/items/{id}/parts/{partId}", func(writer http.ResponseWriter, request *http.Request) error {
		itemID, part, err := decode(writer, request)
		if err != nil {
			return err
		}
		part.ID, err = strconv.ParseInt(request.PathValue("partId"), 10, 64)
		if err != nil {
			return errPartNotFound
		}
		householdID := auth.PrincipalFromRequest(request).HouseholdID
		result, err := s.store.Database.ExecContext(request.Context(), `UPDATE item_parts SET name = ?, part_number = ?, note = ?, source_url = ?, updated_at = ?
			WHERE id = ? AND item_id = ? AND household_id = ? AND `+partOfActiveItem, part.Name, part.PartNumber, part.Note, part.SourceURL, store.Now(), part.ID, itemID, householdID)
		if err != nil {
			return err
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return errPartNotFound
		}
		httpapi.WriteJSON(writer, http.StatusOK, part)
		return nil
	})

	handle("DELETE /api/v1/items/{id}/parts/{partId}", func(writer http.ResponseWriter, request *http.Request) error {
		itemID, err := httpapi.PathID(request)
		if err != nil {
			return err
		}
		partID, err := strconv.ParseInt(request.PathValue("partId"), 10, 64)
		if err != nil {
			return errPartNotFound
		}
		result, err := s.store.Database.ExecContext(request.Context(), `DELETE FROM item_parts WHERE id = ? AND item_id = ? AND household_id = ? AND `+partOfActiveItem,
			partID, itemID, auth.PrincipalFromRequest(request).HouseholdID)
		if err != nil {
			return err
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return errPartNotFound
		}
		writer.WriteHeader(http.StatusNoContent)
		return nil
	})
}
