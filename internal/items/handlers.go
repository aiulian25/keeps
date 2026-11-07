package items

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aiulian25/keeps/internal/auth"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
)

const (
	idempotencyHeader = "Idempotency-Key"
	ifMatchHeader     = "If-Match"
	etagHeader        = "ETag"
	maximumKeyLength  = 100
	maintenanceEvery  = time.Hour
)

var errMissingRevision = httpapi.NewError(http.StatusPreconditionRequired, "revision_required", "Reload the item before saving.")

func writeItem(writer http.ResponseWriter, status int, item Item) {
	writer.Header().Set(etagHeader, strconv.Quote(strconv.FormatInt(item.Revision, 10)))
	httpapi.WriteJSON(writer, status, item)
}

func expectedRevision(request *http.Request) (int64, error) {
	value := strings.Trim(request.Header.Get(ifMatchHeader), `"`)
	revision, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, errMissingRevision
	}
	return revision, nil
}

func flag(request *http.Request, name string) bool {
	return request.URL.Query().Get(name) == "1"
}

func (s *Service) Routes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	handle := func(pattern string, handler httpapi.Handler) {
		mux.Handle(pattern, protect(handler))
	}

	handle("GET /api/v1/items", func(writer http.ResponseWriter, request *http.Request) error {
		query := request.URL.Query()
		text := query.Get("q")
		if len(text) > config.SearchQueryMaximumBytes {
			text = text[:config.SearchQueryMaximumBytes]
		}
		list := ListQuery{Text: text, Unassigned: flag(request, "unassigned"), HasReceipt: flag(request, "hasReceipt"),
			PriceKnown: flag(request, "priceKnown"), Condition: query.Get("condition"), InWarranty: flag(request, "inWarranty"), Deleted: flag(request, "deleted"), Sort: query.Get("sort"), Cursor: query.Get("cursor")}
		list.Limit, _ = strconv.Atoi(query.Get("limit"))
		for name, target := range map[string]*int64{"room": &list.RoomID, "category": &list.CategoryID, "tag": &list.TagID, "collection": &list.CollectionID} {
			*target, _ = httpapi.QueryID(request, name)
		}
		page, err := s.List(request.Context(), auth.PrincipalFromRequest(request).HouseholdID, list)
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusOK, page)
		return nil
	})

	handle("GET /api/v1/items/totals", func(writer http.ResponseWriter, request *http.Request) error {
		totals, err := s.Totals(request.Context(), auth.PrincipalFromRequest(request).HouseholdID)
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusOK, totals)
		return nil
	})

	handle("GET /api/v1/items/room-costs", func(writer http.ResponseWriter, request *http.Request) error {
		costs, err := s.RoomCosts(request.Context(), auth.PrincipalFromRequest(request).HouseholdID)
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusOK, costs)
		return nil
	})

	handle("POST /api/v1/items", func(writer http.ResponseWriter, request *http.Request) error {
		var input Input
		if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &input); err != nil {
			return err
		}
		principal := auth.PrincipalFromRequest(request)
		key := request.Header.Get(idempotencyHeader)
		if len(key) > maximumKeyLength {
			key = key[:maximumKeyLength]
		}
		item, err := s.Create(request.Context(), principal.HouseholdID, principal.UserID, key, input, nil)
		if err != nil {
			return err
		}
		writeItem(writer, http.StatusCreated, item)
		return nil
	})

	handle("GET /api/v1/items/{id}", func(writer http.ResponseWriter, request *http.Request) error {
		identifier, err := httpapi.PathID(request)
		if err != nil {
			return err
		}
		item, err := s.Get(request.Context(), auth.PrincipalFromRequest(request).HouseholdID, identifier)
		if err != nil {
			return err
		}
		writeItem(writer, http.StatusOK, item)
		return nil
	})

	handle("PUT /api/v1/items/{id}", func(writer http.ResponseWriter, request *http.Request) error {
		identifier, err := httpapi.PathID(request)
		if err != nil {
			return err
		}
		revision, err := expectedRevision(request)
		if err != nil {
			return err
		}
		var input Input
		if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &input); err != nil {
			return err
		}
		item, err := s.Update(request.Context(), auth.PrincipalFromRequest(request).HouseholdID, identifier, revision, input)
		if err != nil {
			return err
		}
		writeItem(writer, http.StatusOK, item)
		return nil
	})

	handle("GET /api/v1/items/{id}/history", func(writer http.ResponseWriter, request *http.Request) error {
		identifier, err := httpapi.PathID(request)
		if err != nil {
			return err
		}
		history, err := s.History(request.Context(), auth.PrincipalFromRequest(request).HouseholdID, identifier)
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusOK, history)
		return nil
	})

	itemAction := func(action func(ctx context.Context, householdID int64, itemID int64) error) httpapi.Handler {
		return func(writer http.ResponseWriter, request *http.Request) error {
			identifier, err := httpapi.PathID(request)
			if err != nil {
				return err
			}
			if err := action(request.Context(), auth.PrincipalFromRequest(request).HouseholdID, identifier); err != nil {
				return err
			}
			writer.WriteHeader(http.StatusNoContent)
			return nil
		}
	}
	s.partRoutes(handle)
	s.presetRoutes(handle)
	handle("DELETE /api/v1/items/{id}", itemAction(s.Trash))
	handle("POST /api/v1/items/{id}/restore", itemAction(s.Restore))
	handle("DELETE /api/v1/trash/{id}", func(writer http.ResponseWriter, request *http.Request) error {
		identifier, err := httpapi.PathID(request)
		if err != nil {
			return err
		}
		principal := auth.PrincipalFromRequest(request)
		if err := s.Purge(request.Context(), principal.HouseholdID, principal.UserID, identifier); err != nil {
			return err
		}
		writer.WriteHeader(http.StatusNoContent)
		return nil
	})
}

// RunBackground clears never-attached uploads.
func (s *Service) RunBackground(ctx context.Context) {
	ticker := time.NewTicker(maintenanceEvery)
	defer ticker.Stop()
	for {
		s.PurgeAbandonedAttachments(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
