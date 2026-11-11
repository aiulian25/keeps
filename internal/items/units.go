package items

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/aiulian25/keeps/internal/auth"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/store"
)

// Unit is one of several matching belongings recorded as a single item, such
// as the left one of two speakers. The item keeps its quantity, unit price,
// purchase and receipts; a unit adds only what tells it apart.
type Unit struct {
	ID           int64  `json:"id"`
	Label        string `json:"label"`
	SerialNumber string `json:"serialNumber"`
}

const (
	unitsPerItemMaximum   = 100
	unitSummarySeparator  = "; "
	unitLabelAndSerialGap = ": "
)

var errUnitNotFound = httpapi.NewError(http.StatusNotFound, "unit_not_found", "That unit is no longer recorded for this item, or the item was moved to Recently deleted. Open the item again to see it as it is now.")

// unitOfActiveItem limits a unit change to units whose item is in the same
// household and not in the trash, so a restored item brings back what it had.
const unitOfActiveItem = `EXISTS (SELECT 1 FROM items i WHERE i.id = item_units.item_id AND i.household_id = item_units.household_id AND i.deleted_at IS NULL)`

func validateUnit(input Unit) (Unit, error) {
	unit := Unit{Label: clean(input.Label), SerialNumber: clean(input.SerialNumber)}
	fields := map[string]string{}
	if unit.Label == "" && unit.SerialNumber == "" {
		fields["label"] = "Enter a label, like Left speaker, or the unit's serial number."
	}
	if tooLong(unit.Label, config.ItemShortTextMaximum) {
		fields["label"] = "Use up to 200 characters."
	}
	if tooLong(unit.SerialNumber, config.ItemShortTextMaximum) {
		fields["serialNumber"] = "Use up to 200 characters."
	}
	if len(fields) > 0 {
		return Unit{}, &httpapi.Error{Status: http.StatusBadRequest, Code: "invalid_unit", Message: "Check the highlighted fields.", Fields: fields}
	}
	return unit, nil
}

func loadUnits(ctx context.Context, database querier, itemID int64) ([]Unit, error) {
	rows, err := database.QueryContext(ctx, `SELECT id, label, serial_number FROM item_units WHERE item_id = ? ORDER BY id`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	units := []Unit{}
	for rows.Next() {
		var unit Unit
		if err := rows.Scan(&unit.ID, &unit.Label, &unit.SerialNumber); err != nil {
			return nil, err
		}
		units = append(units, unit)
	}
	return units, rows.Err()
}

// UnitsSummary puts every identified unit in one spreadsheet cell, like
// "Left speaker: SN-1; SN-2".
func UnitsSummary(units []Unit) string {
	described := make([]string, 0, len(units))
	for _, unit := range units {
		named := []string{}
		for _, part := range []string{unit.Label, unit.SerialNumber} {
			if part != "" {
				named = append(named, part)
			}
		}
		described = append(described, strings.Join(named, unitLabelAndSerialGap))
	}
	return strings.Join(described, unitSummarySeparator)
}

// unitsBelowQuantity refuses a quantity lower than the units already
// identified, so the owner chooses which unit to remove instead of Keeps.
func unitsBelowQuantity(quantity int, identified int) error {
	if quantity >= identified {
		return nil
	}
	message := strconv.Itoa(identified) + " units are identified under Individual units. Remove the ones you no longer have there, then lower the quantity."
	return &httpapi.Error{Status: http.StatusBadRequest, Code: "quantity_below_units", Message: "Check the highlighted fields.", Fields: map[string]string{"quantity": message}}
}

// roomForAnotherUnit checks, inside the adding transaction, that the item is
// active and still has a unit left to identify.
func roomForAnotherUnit(ctx context.Context, transaction *sql.Tx, householdID int64, itemID int64) error {
	var quantity, identified int
	err := transaction.QueryRowContext(ctx, `SELECT quantity, (SELECT count(*) FROM item_units WHERE item_id = items.id) FROM items WHERE id = ? AND household_id = ? AND deleted_at IS NULL`,
		itemID, householdID).Scan(&quantity, &identified)
	if err != nil {
		return ErrNotFound
	}
	if identified >= quantity {
		return httpapi.NewError(http.StatusBadRequest, "all_units_identified", "All "+strconv.Itoa(quantity)+" units of this item are identified. Raise its quantity to add another.")
	}
	if identified >= unitsPerItemMaximum {
		return httpapi.NewError(http.StatusBadRequest, "too_many_units", "An item can have up to "+strconv.Itoa(unitsPerItemMaximum)+" identified units.")
	}
	return nil
}

func (s *Service) unitRoutes(handle func(pattern string, handler httpapi.Handler)) {
	decode := func(writer http.ResponseWriter, request *http.Request) (int64, Unit, error) {
		itemID, err := httpapi.PathID(request)
		if err != nil {
			return 0, Unit{}, err
		}
		var input Unit
		if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &input); err != nil {
			return 0, Unit{}, err
		}
		unit, err := validateUnit(input)
		return itemID, unit, err
	}
	// change runs one unit statement and reindexes the item, so a unit's
	// serial number finds it in search.
	change := func(request *http.Request, itemID int64, statement string, arguments ...any) error {
		return s.store.Transaction(request.Context(), func(transaction *sql.Tx) error {
			result, err := transaction.ExecContext(request.Context(), statement, arguments...)
			if err != nil {
				return err
			}
			if changed, _ := result.RowsAffected(); changed != 1 {
				return errUnitNotFound
			}
			return Reindex(request.Context(), transaction, itemID)
		})
	}

	handle("POST /api/v1/items/{id}/units", func(writer http.ResponseWriter, request *http.Request) error {
		itemID, unit, err := decode(writer, request)
		if err != nil {
			return err
		}
		householdID := auth.PrincipalFromRequest(request).HouseholdID
		err = s.store.Transaction(request.Context(), func(transaction *sql.Tx) error {
			if err := roomForAnotherUnit(request.Context(), transaction, householdID, itemID); err != nil {
				return err
			}
			now := store.Now()
			result, err := transaction.ExecContext(request.Context(), `INSERT INTO item_units (household_id, item_id, label, serial_number, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
				householdID, itemID, unit.Label, unit.SerialNumber, now, now)
			if err != nil {
				return err
			}
			if unit.ID, err = result.LastInsertId(); err != nil {
				return err
			}
			return Reindex(request.Context(), transaction, itemID)
		})
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusCreated, unit)
		return nil
	})

	handle("PUT /api/v1/items/{id}/units/{unitId}", func(writer http.ResponseWriter, request *http.Request) error {
		itemID, unit, err := decode(writer, request)
		if err != nil {
			return err
		}
		unit.ID, err = strconv.ParseInt(request.PathValue("unitId"), 10, 64)
		if err != nil {
			return errUnitNotFound
		}
		err = change(request, itemID, `UPDATE item_units SET label = ?, serial_number = ?, updated_at = ?
			WHERE id = ? AND item_id = ? AND household_id = ? AND `+unitOfActiveItem,
			unit.Label, unit.SerialNumber, store.Now(), unit.ID, itemID, auth.PrincipalFromRequest(request).HouseholdID)
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusOK, unit)
		return nil
	})

	handle("DELETE /api/v1/items/{id}/units/{unitId}", func(writer http.ResponseWriter, request *http.Request) error {
		itemID, err := httpapi.PathID(request)
		if err != nil {
			return err
		}
		unitID, err := strconv.ParseInt(request.PathValue("unitId"), 10, 64)
		if err != nil {
			return errUnitNotFound
		}
		err = change(request, itemID, `DELETE FROM item_units WHERE id = ? AND item_id = ? AND household_id = ? AND `+unitOfActiveItem,
			unitID, itemID, auth.PrincipalFromRequest(request).HouseholdID)
		if err != nil {
			return err
		}
		writer.WriteHeader(http.StatusNoContent)
		return nil
	})
}
