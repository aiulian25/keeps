// Package organise manages the optional groupings: rooms, categories, tags
// and collections. Deleting a grouping never deletes items.
package organise

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/aiulian25/keeps/internal/auth"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/store"
)

type Kind string

const (
	Rooms       Kind = "rooms"
	Categories  Kind = "categories"
	Tags        Kind = "tags"
	Collections Kind = "collections"
)

const thumbnailsPerGroup = 4

// kindDefinition holds the table names for one grouping kind. Table names
// come only from this fixed map, never from user input.
type kindDefinition struct {
	table        string
	joinTable    string
	joinColumn   string
	itemColumn   string
	singularName string
}

var definitions = map[Kind]kindDefinition{
	Rooms:       {table: "rooms", itemColumn: "room_id", singularName: "room"},
	Categories:  {table: "categories", itemColumn: "category_id", singularName: "category"},
	Tags:        {table: "tags", joinTable: "item_tags", joinColumn: "tag_id", singularName: "tag"},
	Collections: {table: "collections", joinTable: "item_collections", joinColumn: "collection_id", singularName: "collection"},
}

var (
	ErrUnknownKind   = httpapi.ErrNotFound
	ErrInvalidName   = httpapi.FieldError("name", "invalid_name", "Enter a name up to 80 characters.")
	ErrDuplicateName = httpapi.FieldError("name", "duplicate_name", "That name is already used.")
)

type Group struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	ItemCount    int     `json:"itemCount"`
	ThumbnailIDs []int64 `json:"thumbnailIds"`
	Description  string  `json:"description,omitempty"`
	PhotoID      int64   `json:"photoId,omitempty"`
}

type Reference struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ReindexFunc refreshes the search index for the given items inside the
// transaction that changed them, so the index never disagrees with the data.
type ReindexFunc func(ctx context.Context, transaction *sql.Tx, itemIDs []int64) error

type Service struct {
	store   *store.Store
	reindex ReindexFunc
}

func NewService(database *store.Store, reindex ReindexFunc) *Service {
	return &Service{store: database, reindex: reindex}
}

func definitionFor(kind string) (Kind, kindDefinition, error) {
	definition, found := definitions[Kind(kind)]
	if !found {
		return "", kindDefinition{}, ErrUnknownKind
	}
	return Kind(kind), definition, nil
}

// CleanName trims and validates a grouping name.
func CleanName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || utf8.RuneCountInString(name) > config.GroupNameMaximumLength {
		return "", ErrInvalidName
	}
	return name, nil
}

// itemMembership returns SQL selecting the item IDs in a group.
func itemMembership(definition kindDefinition) string {
	if definition.joinTable != "" {
		return `SELECT item_id FROM ` + definition.joinTable + ` WHERE ` + definition.joinColumn + ` = g.id`
	}
	return `SELECT id FROM items WHERE ` + definition.itemColumn + ` = g.id`
}

func (s *Service) List(ctx context.Context, householdID int64, kind string) ([]Group, error) {
	_, definition, err := definitionFor(kind)
	if err != nil {
		return nil, err
	}
	membership := itemMembership(definition)
	rows, err := s.store.Database.QueryContext(ctx, `SELECT g.id, g.name,
			(SELECT count(*) FROM items i WHERE i.deleted_at IS NULL AND i.id IN (`+membership+`))
		FROM `+definition.table+` g WHERE g.household_id = ? ORDER BY g.name COLLATE NOCASE`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := []Group{}
	for rows.Next() {
		group := Group{ThumbnailIDs: []int64{}}
		if err := rows.Scan(&group.ID, &group.Name, &group.ItemCount); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range groups {
		groups[index].ThumbnailIDs, err = s.thumbnails(ctx, definition, groups[index].ID)
		if err != nil {
			return nil, err
		}
	}
	if Kind(kind) != Rooms {
		return groups, nil
	}
	details, err := s.roomDetails(ctx, householdID)
	if err != nil {
		return nil, err
	}
	for index := range groups {
		groups[index].Description = details[groups[index].ID].Description
		groups[index].PhotoID = details[groups[index].ID].PhotoID
	}
	return groups, nil
}

func (s *Service) thumbnails(ctx context.Context, definition kindDefinition, groupID int64) ([]int64, error) {
	membership := strings.Replace(itemMembership(definition), "g.id", "?", 1)
	rows, err := s.store.Database.QueryContext(ctx, `SELECT primary_photo_id FROM items
		WHERE deleted_at IS NULL AND primary_photo_id IS NOT NULL AND id IN (`+membership+`)
		ORDER BY created_at DESC LIMIT ?`, groupID, thumbnailsPerGroup)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	identifiers := []int64{}
	for rows.Next() {
		var identifier int64
		if err := rows.Scan(&identifier); err != nil {
			return nil, err
		}
		identifiers = append(identifiers, identifier)
	}
	return identifiers, rows.Err()
}

// Create adds a grouping inside transaction.
func (s *Service) Create(ctx context.Context, transaction *sql.Tx, householdID int64, kind string, name string) (Reference, error) {
	_, definition, err := definitionFor(kind)
	if err != nil {
		return Reference{}, err
	}
	name, err = CleanName(name)
	if err != nil {
		return Reference{}, err
	}
	result, err := transaction.ExecContext(ctx, `INSERT INTO `+definition.table+` (household_id, name, created_at) VALUES (?, ?, ?)`, householdID, name, store.Now())
	if store.IsUniqueViolation(err) {
		return Reference{}, ErrDuplicateName
	}
	if err != nil {
		return Reference{}, err
	}
	identifier, _ := result.LastInsertId()
	return Reference{ID: identifier, Name: name}, nil
}

// Rename renames a grouping and reindexes its items inside transaction.
func (s *Service) Rename(ctx context.Context, transaction *sql.Tx, householdID int64, kind string, groupID int64, name string) (Reference, error) {
	_, definition, err := definitionFor(kind)
	if err != nil {
		return Reference{}, err
	}
	name, err = CleanName(name)
	if err != nil {
		return Reference{}, err
	}
	result, err := transaction.ExecContext(ctx, `UPDATE `+definition.table+` SET name = ? WHERE id = ? AND household_id = ?`, name, groupID, householdID)
	if store.IsUniqueViolation(err) {
		return Reference{}, ErrDuplicateName
	}
	if err != nil {
		return Reference{}, err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return Reference{}, httpapi.ErrNotFound
	}
	return Reference{ID: groupID, Name: name}, s.reindexGroup(ctx, transaction, definition, groupID)
}

// Delete removes a grouping. Its items are left unassigned, or moved to
// reassignTo when given (rooms and categories only).
func (s *Service) Delete(ctx context.Context, householdID int64, kind string, groupID int64, reassignTo int64) error {
	_, definition, err := definitionFor(kind)
	if err != nil {
		return err
	}
	return s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		affected, err := s.memberIDs(ctx, transaction, definition, groupID)
		if err != nil {
			return err
		}
		if reassignTo != 0 && definition.itemColumn != "" {
			if err := ownsGroup(ctx, transaction, definition, householdID, reassignTo); err != nil {
				return err
			}
			_, err := transaction.ExecContext(ctx, `UPDATE items SET `+definition.itemColumn+` = ?, revision = revision + 1, updated_at = ? WHERE `+definition.itemColumn+` = ? AND household_id = ?`,
				reassignTo, store.Now(), groupID, householdID)
			if err != nil {
				return err
			}
		}
		result, err := transaction.ExecContext(ctx, `DELETE FROM `+definition.table+` WHERE id = ? AND household_id = ?`, groupID, householdID)
		if err != nil {
			return err
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return httpapi.ErrNotFound
		}
		return s.reindexItems(ctx, transaction, affected)
	})
}

func ownsGroup(ctx context.Context, transaction *sql.Tx, definition kindDefinition, householdID int64, groupID int64) error {
	var found int
	err := transaction.QueryRowContext(ctx, `SELECT 1 FROM `+definition.table+` WHERE id = ? AND household_id = ?`, groupID, householdID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return httpapi.FieldError("reassignTo", "not_found", "Choose another "+definition.singularName+".")
	}
	return err
}

func (s *Service) memberIDs(ctx context.Context, transaction *sql.Tx, definition kindDefinition, groupID int64) ([]int64, error) {
	query := strings.Replace(itemMembership(definition), "g.id", "?", 1)
	rows, err := transaction.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var identifiers []int64
	for rows.Next() {
		var identifier int64
		if err := rows.Scan(&identifier); err != nil {
			return nil, err
		}
		identifiers = append(identifiers, identifier)
	}
	return identifiers, rows.Err()
}

func (s *Service) reindexGroup(ctx context.Context, transaction *sql.Tx, definition kindDefinition, groupID int64) error {
	identifiers, err := s.memberIDs(ctx, transaction, definition, groupID)
	if err != nil {
		return err
	}
	return s.reindexItems(ctx, transaction, identifiers)
}

func (s *Service) reindexItems(ctx context.Context, transaction *sql.Tx, identifiers []int64) error {
	if len(identifiers) == 0 {
		return nil
	}
	return s.reindex(ctx, transaction, identifiers)
}

// ResolveByName finds or creates a grouping by name inside a transaction, for
// inline creation while editing an item. An empty name resolves to none (0).
func ResolveByName(ctx context.Context, transaction *sql.Tx, householdID int64, kind Kind, name string) (int64, error) {
	if strings.TrimSpace(name) == "" {
		return 0, nil
	}
	name, err := CleanName(name)
	if err != nil {
		return 0, err
	}
	definition := definitions[kind]
	var identifier int64
	err = transaction.QueryRowContext(ctx, `SELECT id FROM `+definition.table+` WHERE household_id = ? AND name = ?`, householdID, name).Scan(&identifier)
	if err == nil {
		return identifier, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	result, err := transaction.ExecContext(ctx, `INSERT INTO `+definition.table+` (household_id, name, created_at) VALUES (?, ?, ?)`, householdID, name, store.Now())
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// Routes registers /api/v1/{kind} endpoints for every grouping kind.
func (s *Service) Routes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	handle := func(pattern string, handler httpapi.Handler) {
		mux.Handle(pattern, protect(handler))
	}
	handle("GET /api/v1/groups/{kind}", func(writer http.ResponseWriter, request *http.Request) error {
		groups, err := s.List(request.Context(), auth.PrincipalFromRequest(request).HouseholdID, request.PathValue("kind"))
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusOK, groups)
		return nil
	})
	handle("POST /api/v1/groups/{kind}", func(writer http.ResponseWriter, request *http.Request) error {
		var body struct {
			Name string `json:"name"`
			RoomDetails
		}
		if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &body); err != nil {
			return err
		}
		householdID := auth.PrincipalFromRequest(request).HouseholdID
		kind := request.PathValue("kind")
		if !body.RoomDetails.empty() && Kind(kind) != Rooms {
			return ErrDetailsRoomsOnly
		}
		var created Reference
		err := s.store.Transaction(request.Context(), func(transaction *sql.Tx) error {
			var err error
			created, err = s.Create(request.Context(), transaction, householdID, kind, body.Name)
			if err != nil {
				return err
			}
			return s.UpdateRoomDetails(request.Context(), transaction, householdID, kind, created.ID, body.RoomDetails)
		})
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusCreated, created)
		return nil
	})
	handle("PATCH /api/v1/groups/{kind}/{id}", func(writer http.ResponseWriter, request *http.Request) error {
		identifier, err := httpapi.PathID(request)
		if err != nil {
			return err
		}
		var body struct {
			Name *string `json:"name"`
			RoomDetails
		}
		if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &body); err != nil {
			return err
		}
		householdID := auth.PrincipalFromRequest(request).HouseholdID
		kind := request.PathValue("kind")
		reference := Reference{ID: identifier}
		err = s.store.Transaction(request.Context(), func(transaction *sql.Tx) error {
			if body.Name != nil {
				renamed, err := s.Rename(request.Context(), transaction, householdID, kind, identifier, *body.Name)
				if err != nil {
					return err
				}
				reference = renamed
			}
			return s.UpdateRoomDetails(request.Context(), transaction, householdID, kind, identifier, body.RoomDetails)
		})
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusOK, reference)
		return nil
	})
	handle("DELETE /api/v1/groups/{kind}/{id}", func(writer http.ResponseWriter, request *http.Request) error {
		identifier, err := httpapi.PathID(request)
		if err != nil {
			return err
		}
		reassignTo, _ := httpapi.QueryID(request, "reassignTo")
		if err := s.Delete(request.Context(), auth.PrincipalFromRequest(request).HouseholdID, request.PathValue("kind"), identifier, reassignTo); err != nil {
			return err
		}
		writer.WriteHeader(http.StatusNoContent)
		return nil
	})
}
