package items

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/aiulian25/keeps/internal/auth"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/store"
)

// Preset is a reusable set of starting details for similar belongings. It
// carries organising details and generic notes only, so it can never copy
// a serial number, price, purchase or receipt onto another item.
type Preset struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Room        string   `json:"room"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Collections []string `json:"collections"`
	Condition   string   `json:"condition"`
	Notes       string   `json:"notes"`
	// Missing names groups the preset refers to that don't exist (yet or any
	// more); saving an item with the preset creates them.
	Missing []string `json:"missing"`
}

const presetsPerHousehold = 50

var (
	errPresetNotFound  = httpapi.NewError(http.StatusNotFound, "preset_not_found", "That preset no longer exists.")
	errPresetDuplicate = httpapi.FieldError("name", "preset_exists", "A preset with that name already exists.")
)

func validatePreset(input Preset) (Preset, error) {
	preset := Preset{
		Name: clean(input.Name), Room: clean(input.Room), Category: clean(input.Category),
		Tags: uniqueNames(input.Tags), Collections: uniqueNames(input.Collections),
		Condition: clean(input.Condition), Notes: strings.TrimRight(input.Notes, " \n\t"),
	}
	fields := map[string]string{}
	if preset.Name == "" || tooLong(preset.Name, config.ItemShortTextMaximum) {
		fields["name"] = "Give the preset a name, up to 200 characters."
	}
	if !Conditions[preset.Condition] {
		fields["condition"] = "Choose a condition from the list."
	}
	if tooLong(preset.Notes, config.ItemTextMaximumLength) {
		fields["notes"] = "This is too long."
	}
	if len(preset.Tags) > maximumGroupNames || len(preset.Collections) > maximumGroupNames {
		fields["tags"] = "Too many tags or collections."
	}
	empty := preset.Room == "" && preset.Category == "" && len(preset.Tags) == 0 && len(preset.Collections) == 0 && preset.Condition == "" && preset.Notes == ""
	if empty {
		fields["name"] = "Fill in a room, category, tags, collections, condition or notes first."
	}
	if len(fields) > 0 {
		return Preset{}, &httpapi.Error{Status: http.StatusBadRequest, Code: "invalid_preset", Message: "Check the preset.", Fields: fields}
	}
	return preset, nil
}

func (s *Service) listPresets(ctx context.Context, householdID int64) ([]Preset, error) {
	rows, err := s.store.Database.QueryContext(ctx, `SELECT id, name, room, category, tags_json, collections_json, condition, notes FROM capture_presets WHERE household_id = ? ORDER BY name`, householdID)
	if err != nil {
		return nil, err
	}
	presets := []Preset{}
	for rows.Next() {
		var preset Preset
		var tags, collections string
		if err := rows.Scan(&preset.ID, &preset.Name, &preset.Room, &preset.Category, &tags, &collections, &preset.Condition, &preset.Notes); err != nil {
			rows.Close()
			return nil, err
		}
		json.Unmarshal([]byte(tags), &preset.Tags)
		json.Unmarshal([]byte(collections), &preset.Collections)
		presets = append(presets, preset)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range presets {
		presets[index].Missing, err = s.missingGroups(ctx, householdID, presets[index])
		if err != nil {
			return nil, err
		}
	}
	return presets, nil
}

func (s *Service) missingGroups(ctx context.Context, householdID int64, preset Preset) ([]string, error) {
	missing := []string{}
	check := func(table string, name string) error {
		if name == "" {
			return nil
		}
		var found int
		err := s.store.Database.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE household_id = ? AND name = ?`, householdID, name).Scan(&found)
		if err == nil && found == 0 {
			missing = append(missing, name)
		}
		return err
	}
	if err := check("rooms", preset.Room); err != nil {
		return nil, err
	}
	if err := check("categories", preset.Category); err != nil {
		return nil, err
	}
	for _, tag := range preset.Tags {
		if err := check("tags", tag); err != nil {
			return nil, err
		}
	}
	for _, collection := range preset.Collections {
		if err := check("collections", collection); err != nil {
			return nil, err
		}
	}
	return missing, nil
}

func (s *Service) presetRoutes(handle func(pattern string, handler httpapi.Handler)) {
	handle("GET /api/v1/presets", func(writer http.ResponseWriter, request *http.Request) error {
		presets, err := s.listPresets(request.Context(), auth.PrincipalFromRequest(request).HouseholdID)
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusOK, presets)
		return nil
	})

	handle("POST /api/v1/presets", func(writer http.ResponseWriter, request *http.Request) error {
		var input Preset
		if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &input); err != nil {
			return err
		}
		preset, err := validatePreset(input)
		if err != nil {
			return err
		}
		householdID := auth.PrincipalFromRequest(request).HouseholdID
		var count int
		s.store.Database.QueryRowContext(request.Context(), `SELECT count(*) FROM capture_presets WHERE household_id = ?`, householdID).Scan(&count)
		if count >= presetsPerHousehold {
			return httpapi.NewError(http.StatusBadRequest, "too_many_presets", "You can keep up to "+strconv.Itoa(presetsPerHousehold)+" presets.")
		}
		tags, _ := json.Marshal(preset.Tags)
		collections, _ := json.Marshal(preset.Collections)
		result, err := s.store.Database.ExecContext(request.Context(), `INSERT INTO capture_presets (household_id, name, room, category, tags_json, collections_json, condition, notes, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, householdID, preset.Name, preset.Room, preset.Category, string(tags), string(collections), preset.Condition, preset.Notes, store.Now())
		if store.IsUniqueViolation(err) {
			return errPresetDuplicate
		}
		if err != nil {
			return err
		}
		preset.ID, _ = result.LastInsertId()
		preset.Missing = []string{}
		httpapi.WriteJSON(writer, http.StatusCreated, preset)
		return nil
	})

	handle("DELETE /api/v1/presets/{id}", func(writer http.ResponseWriter, request *http.Request) error {
		identifier, err := httpapi.PathID(request)
		if err != nil {
			return err
		}
		result, err := s.store.Database.ExecContext(request.Context(), `DELETE FROM capture_presets WHERE id = ? AND household_id = ?`, identifier, auth.PrincipalFromRequest(request).HouseholdID)
		if err != nil {
			return err
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return errPresetNotFound
		}
		writer.WriteHeader(http.StatusNoContent)
		return nil
	})
}
