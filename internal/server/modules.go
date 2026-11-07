package server

import (
	"context"
	"database/sql"

	"github.com/aiulian25/keeps/internal/attachments"
	"github.com/aiulian25/keeps/internal/importer"
	"github.com/aiulian25/keeps/internal/items"
	"github.com/aiulian25/keeps/internal/organise"
	"github.com/aiulian25/keeps/internal/safefetch"
)

// registerModules adds every feature area to the application.
func (a *Application) registerModules() {
	itemService := items.NewService(a.Store)
	a.Items = itemService
	a.AddModule(itemService)
	a.AddModule(organise.NewService(a.Store, func(ctx context.Context, transaction *sql.Tx, itemIDs []int64) error {
		return items.Reindex(ctx, transaction, itemIDs...)
	}))
	a.Attachments = attachments.NewService(a.Store, a.Blobs, a.Configuration.DataDirectory)
	a.AddModule(a.Attachments)
	a.Importer = importer.NewService(a.Store, safefetch.New(), a.Attachments, itemService)
	a.AddModule(a.Importer)
}
