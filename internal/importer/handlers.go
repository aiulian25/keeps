package importer

import (
	"net/http"

	"github.com/aiulian25/keeps/internal/auth"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/items"
)

const idempotencyHeader = "Idempotency-Key"

func (s *Service) Routes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	handle := func(pattern string, handler httpapi.Handler) {
		mux.Handle(pattern, protect(handler))
	}
	handle("POST /api/v1/imports", func(writer http.ResponseWriter, request *http.Request) error {
		var body struct {
			URL string `json:"url"`
		}
		if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &body); err != nil {
			return err
		}
		principal := auth.PrincipalFromRequest(request)
		draft, err := s.Start(request.Context(), principal.HouseholdID, principal.UserID, body.URL)
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusAccepted, draft)
		return nil
	})
	handle("POST /api/v1/imports/page", func(writer http.ResponseWriter, request *http.Request) error {
		var snapshot PageSnapshot
		if err := httpapi.DecodeJSON(writer, request, PageSnapshotBodyLimit, &snapshot); err != nil {
			return err
		}
		principal := auth.PrincipalFromRequest(request)
		draft, err := s.StartFromPage(request.Context(), principal.HouseholdID, principal.UserID, snapshot)
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusAccepted, draft)
		return nil
	})
	handle("GET /api/v1/imports/{id}", func(writer http.ResponseWriter, request *http.Request) error {
		draft, err := s.Get(request.Context(), auth.PrincipalFromRequest(request).HouseholdID, request.PathValue("id"))
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusOK, draft)
		return nil
	})
	handle("DELETE /api/v1/imports/{id}", func(writer http.ResponseWriter, request *http.Request) error {
		if err := s.Cancel(request.Context(), auth.PrincipalFromRequest(request).HouseholdID, request.PathValue("id")); err != nil {
			return err
		}
		writer.WriteHeader(http.StatusNoContent)
		return nil
	})
	handle("POST /api/v1/imports/{id}/save", func(writer http.ResponseWriter, request *http.Request) error {
		var body struct {
			Candidate int         `json:"candidate"`
			Item      items.Input `json:"item"`
		}
		if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &body); err != nil {
			return err
		}
		principal := auth.PrincipalFromRequest(request)
		item, err := s.Save(request.Context(), principal.HouseholdID, principal.UserID, request.PathValue("id"), body.Candidate, body.Item, request.Header.Get(idempotencyHeader))
		if err != nil {
			return err
		}
		httpapi.WriteJSON(writer, http.StatusCreated, item)
		return nil
	})
}
