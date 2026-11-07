package organise

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/aiulian25/keeps/internal/httpapi"
)

const roomDescriptionMaximumLength = 1000

var (
	ErrDescriptionTooLong = httpapi.FieldError("description", "description_too_long", "Use up to 1000 characters.")
	ErrRoomPhoto          = httpapi.FieldError("photoId", "invalid_photo", "That photo is no longer available. Upload it again.")
	ErrDetailsRoomsOnly   = httpapi.NewError(http.StatusBadRequest, "rooms_only", "Only rooms have a description and photo.")
)

// RoomDetails are the optional extras a room can have. A nil field is left
// unchanged; a photo ID of 0 removes the photo.
type RoomDetails struct {
	Description *string `json:"description"`
	PhotoID     *int64  `json:"photoId"`
}

func (details RoomDetails) empty() bool {
	return details.Description == nil && details.PhotoID == nil
}

// UpdateRoomDetails sets a room's description and photo inside transaction.
// Everything is checked before anything is written, so a bad photo never
// leaves a new description behind. The photo must be a photo upload
// belonging to the same household.
func (s *Service) UpdateRoomDetails(ctx context.Context, transaction *sql.Tx, householdID int64, kind string, roomID int64, details RoomDetails) error {
	if details.empty() {
		return nil
	}
	if Kind(kind) != Rooms {
		return ErrDetailsRoomsOnly
	}
	description := ""
	if details.Description != nil {
		description = strings.TrimSpace(*details.Description)
		if utf8.RuneCountInString(description) > roomDescriptionMaximumLength {
			return ErrDescriptionTooLong
		}
	}
	photo := sql.NullInt64{}
	if details.PhotoID != nil {
		photo = sql.NullInt64{Int64: *details.PhotoID, Valid: *details.PhotoID != 0}
	}
	if photo.Valid {
		var found int
		err := transaction.QueryRowContext(ctx, `SELECT 1 FROM attachments WHERE id = ? AND household_id = ? AND kind = 'photo'`, photo.Int64, householdID).Scan(&found)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRoomPhoto
		}
		if err != nil {
			return err
		}
	}
	if details.Description != nil {
		result, err := transaction.ExecContext(ctx, `UPDATE rooms SET description = ? WHERE id = ? AND household_id = ?`, description, roomID, householdID)
		if err := requireChanged(result, err); err != nil {
			return err
		}
	}
	if details.PhotoID == nil {
		return nil
	}
	result, err := transaction.ExecContext(ctx, `UPDATE rooms SET photo_id = ? WHERE id = ? AND household_id = ?`, photo, roomID, householdID)
	return requireChanged(result, err)
}

func requireChanged(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return httpapi.ErrNotFound
	}
	return nil
}

// roomDetails returns description and photo for every room in a household.
func (s *Service) roomDetails(ctx context.Context, householdID int64) (map[int64]Group, error) {
	rows, err := s.store.Database.QueryContext(ctx, `SELECT id, description, coalesce(photo_id, 0) FROM rooms WHERE household_id = ?`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	details := map[int64]Group{}
	for rows.Next() {
		var group Group
		if err := rows.Scan(&group.ID, &group.Description, &group.PhotoID); err != nil {
			return nil, err
		}
		details[group.ID] = group
	}
	return details, rows.Err()
}
