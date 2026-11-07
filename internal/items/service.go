package items

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aiulian25/keeps/internal/audit"
	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/money"
	"github.com/aiulian25/keeps/internal/organise"
	"github.com/aiulian25/keeps/internal/store"
)

const (
	sourceUser   = "user"
	sourceImport = "import"
	sortNewest   = "newest"
	sortPurchase = "purchased"
	sortName     = "name"
)

var (
	ErrNotFound = httpapi.ErrNotFound
	ErrConflict = httpapi.NewError(409, "edit_conflict", "This item was changed somewhere else. Reload to see the latest version.")
)

type Service struct {
	store *store.Store
}

func NewService(database *store.Store) *Service {
	return &Service{store: database}
}

func (s *Service) householdCurrency(ctx context.Context, householdID int64) string {
	currency := "GBP"
	s.store.Database.QueryRowContext(ctx, `SELECT currency FROM households WHERE id = ?`, householdID).Scan(&currency)
	return currency
}

func (s *Service) Get(ctx context.Context, householdID int64, itemID int64) (Item, error) {
	item, err := loadItem(ctx, s.store.Database, householdID, itemID)
	if errors.Is(err, errItemNotFound) {
		return Item{}, ErrNotFound
	}
	return item, err
}

// Create saves a new item. Only the name is required. Repeating a request
// with the same idempotency key returns the item created the first time.
func (s *Service) Create(ctx context.Context, householdID int64, userID int64, idempotencyKey string, input Input, origin *ImportOrigin) (Item, error) {
	if existing, found := s.previousResult(ctx, userID, idempotencyKey); found {
		return s.Get(ctx, householdID, existing)
	}
	values, err := validate(input, s.householdCurrency(ctx, householdID))
	if err != nil {
		return Item{}, err
	}

	var itemID int64
	err = s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		purchaseID, sharedChanged, err := writePurchase(ctx, transaction, householdID, 0, values)
		if err != nil {
			return err
		}
		roomID, categoryID, err := resolveSingleGroups(ctx, transaction, householdID, values)
		if err != nil {
			return err
		}
		now := store.Now()
		sourceURL := ""
		if origin != nil {
			sourceURL = origin.SourceURL
		}
		result, err := transaction.ExecContext(ctx, `INSERT INTO items (household_id, name, description, quantity, brand, model, serial_number, notes,
				room_id, category_id, purchase_id, paid_unit_minor, paid_currency, source_url, condition, warranty_until, measurements_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			householdID, values.Name, values.Description, values.Quantity, values.Brand, values.Model, values.SerialNumber, values.Notes,
			nullableID(roomID), nullableID(categoryID), nullableID(purchaseID), values.paidMinor, nullableText(values.paidCurrency), sourceURL,
			values.Condition, nullableText(values.WarrantyUntil), encodeMeasurements(values.Measurements), now, now)
		if err != nil {
			return err
		}
		itemID, _ = result.LastInsertId()
		if sharedChanged {
			if err := touchSiblings(ctx, transaction, purchaseID, itemID); err != nil {
				return err
			}
		}
		if err := writeGroups(ctx, transaction, householdID, itemID, values); err != nil {
			return err
		}
		if err := linkAttachments(ctx, transaction, householdID, itemID, purchaseID, values.AttachmentIDs); err != nil {
			return err
		}
		if err := recordProvenance(ctx, transaction, itemID, values, origin); err != nil {
			return err
		}
		source := sourceUser
		if origin != nil {
			source = sourceImport
		}
		if err := recordRevision(ctx, transaction, itemID, source, map[string]any{"created": true}); err != nil {
			return err
		}
		if err := rememberResult(ctx, transaction, userID, idempotencyKey, itemID); err != nil {
			return err
		}
		return Reindex(ctx, transaction, itemID)
	})
	if err != nil {
		return Item{}, err
	}
	item, err := s.Get(ctx, householdID, itemID)
	if err != nil {
		return Item{}, err
	}
	item.PossibleDuplicates, err = s.possibleDuplicates(ctx, householdID, item)
	return item, err
}

func nullableText(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func resolveSingleGroups(ctx context.Context, transaction *sql.Tx, householdID int64, values normalised) (int64, int64, error) {
	roomID, err := organise.ResolveByName(ctx, transaction, householdID, organise.Rooms, values.Room)
	if err != nil {
		return 0, 0, err
	}
	categoryID, err := organise.ResolveByName(ctx, transaction, householdID, organise.Categories, values.Category)
	return roomID, categoryID, err
}

// Update replaces all user-editable fields if expectedRevision matches; a
// stale revision returns ErrConflict instead of silently overwriting.
func (s *Service) Update(ctx context.Context, householdID int64, itemID int64, expectedRevision int64, input Input) (Item, error) {
	values, err := validate(input, s.householdCurrency(ctx, householdID))
	if err != nil {
		return Item{}, err
	}
	err = s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		before, err := loadItem(ctx, transaction, householdID, itemID)
		if errors.Is(err, errItemNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if before.Revision != expectedRevision {
			return ErrConflict
		}
		purchaseID, sharedChanged, err := writePurchase(ctx, transaction, householdID, before.PurchaseID, values)
		if err != nil {
			return err
		}
		roomID, categoryID, err := resolveSingleGroups(ctx, transaction, householdID, values)
		if err != nil {
			return err
		}
		_, err = transaction.ExecContext(ctx, `UPDATE items SET name = ?, description = ?, quantity = ?, brand = ?, model = ?, serial_number = ?, notes = ?,
				room_id = ?, category_id = ?, purchase_id = ?, paid_unit_minor = ?, paid_currency = ?, condition = ?, warranty_until = ?, measurements_json = ?,
				revision = revision + 1, updated_at = ?
			WHERE id = ? AND household_id = ?`,
			values.Name, values.Description, values.Quantity, values.Brand, values.Model, values.SerialNumber, values.Notes,
			nullableID(roomID), nullableID(categoryID), nullableID(purchaseID), values.paidMinor, nullableText(values.paidCurrency),
			values.Condition, nullableText(values.WarrantyUntil), encodeMeasurements(values.Measurements), store.Now(),
			itemID, householdID)
		if err != nil {
			return err
		}
		if sharedChanged {
			if err := touchSiblings(ctx, transaction, purchaseID, itemID); err != nil {
				return err
			}
		}
		if err := writeGroups(ctx, transaction, householdID, itemID, values); err != nil {
			return err
		}
		if err := linkAttachments(ctx, transaction, householdID, itemID, purchaseID, values.AttachmentIDs); err != nil {
			return err
		}
		after, err := loadItem(ctx, transaction, householdID, itemID)
		if err != nil {
			return err
		}
		changes := differences(before, after)
		for field := range changes {
			if err := setProvenance(ctx, transaction, itemID, field, sourceUser, 0); err != nil {
				return err
			}
		}
		if len(changes) > 0 {
			if err := recordRevision(ctx, transaction, itemID, sourceUser, changes); err != nil {
				return err
			}
		}
		affected := append([]int64{itemID}, referenceIDs(after.SharedWith)...)
		return Reindex(ctx, transaction, affected...)
	})
	if err != nil {
		return Item{}, err
	}
	return s.Get(ctx, householdID, itemID)
}

func referenceIDs(references []Reference) []int64 {
	identifiers := make([]int64, 0, len(references))
	for _, reference := range references {
		identifiers = append(identifiers, reference.ID)
	}
	return identifiers
}

// differences lists user-facing fields that changed, for history.
func differences(before Item, after Item) map[string]any {
	describe := func(item Item) map[string]string {
		price := ""
		if item.PricePaid != nil {
			price = item.PricePaid.Amount + " " + item.PricePaid.Currency
		}
		date := ""
		if item.PurchaseDate != nil {
			date = item.PurchaseDate.Value
		}
		name := func(reference *Reference) string {
			if reference == nil {
				return ""
			}
			return reference.Name
		}
		names := func(references []Reference) string {
			list := make([]string, 0, len(references))
			for _, reference := range references {
				list = append(list, reference.Name)
			}
			return strings.Join(list, ", ")
		}
		return map[string]string{
			"name": item.Name, "description": item.Description, "quantity": fmt.Sprint(item.Quantity), "brand": item.Brand,
			"model": item.Model, "serialNumber": item.SerialNumber, "notes": item.Notes, "seller": item.Seller,
			"condition": item.Condition, "warrantyUntil": item.WarrantyUntil, "measurements": item.Measurements.Summary(),
			"orderReference": item.OrderReference, "pricePaid": price, "purchaseDate": date, "room": name(item.Room),
			"category": name(item.Category), "tags": names(item.Tags), "collections": names(item.Collections),
			"attachments": fmt.Sprint(len(item.Photos) + len(item.Documents)),
		}
	}
	old, current := describe(before), describe(after)
	changes := map[string]any{}
	for field, value := range current {
		if old[field] != value {
			changes[field] = map[string]string{"from": old[field], "to": value}
		}
	}
	return changes
}

func recordRevision(ctx context.Context, transaction *sql.Tx, itemID int64, source string, changes map[string]any) error {
	encoded, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	_, err = transaction.ExecContext(ctx, `INSERT INTO item_revisions (item_id, source, changes_json, changed_at) VALUES (?, ?, ?, ?)`, itemID, source, string(encoded), store.Now())
	return err
}

func setProvenance(ctx context.Context, transaction *sql.Tx, itemID int64, field string, source string, snapshotID int64) error {
	_, err := transaction.ExecContext(ctx, `INSERT INTO field_provenance (item_id, field, source_kind, snapshot_id, recorded_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(item_id, field) DO UPDATE SET source_kind = excluded.source_kind, snapshot_id = excluded.snapshot_id, recorded_at = excluded.recorded_at`,
		itemID, field, source, nullableID(snapshotID), store.Now())
	return err
}

// recordProvenance stores the listing snapshot for imports and marks each
// field as imported only if the saved value is still the imported one.
func recordProvenance(ctx context.Context, transaction *sql.Tx, itemID int64, values normalised, origin *ImportOrigin) error {
	if origin == nil {
		return nil
	}
	currency := sql.NullString{String: origin.ListingCurrency, Valid: origin.ListingPrice != nil}
	result, err := transaction.ExecContext(ctx, `INSERT INTO listing_snapshots (item_id, source_url, source_host, title, price_minor, currency, retrieved_at, extractor_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, itemID, origin.SourceURL, origin.SourceHost, origin.Title, origin.ListingPrice, currency, origin.RetrievedAt, origin.ExtractorVersion)
	if err != nil {
		return err
	}
	snapshotID, _ := result.LastInsertId()
	saved := map[string]string{"name": values.Name, "description": values.Description, "brand": values.Brand, "model": values.Model}
	for field, imported := range origin.ImportedValues {
		source := sourceUser
		if saved[field] == imported && imported != "" {
			source = sourceImport
		}
		if err := setProvenance(ctx, transaction, itemID, field, source, snapshotID); err != nil {
			return err
		}
	}
	return nil
}

// linkAttachments attaches uploaded files: invoices to the purchase (so all
// items sharing it see them), photos and documents to the item.
func linkAttachments(ctx context.Context, transaction *sql.Tx, householdID int64, itemID int64, purchaseID int64, attachmentIDs []int64) error {
	for _, attachmentID := range attachmentIDs {
		var kind string
		err := transaction.QueryRowContext(ctx, `SELECT kind FROM attachments WHERE id = ? AND household_id = ?`, attachmentID, householdID).Scan(&kind)
		if errors.Is(err, sql.ErrNoRows) {
			return httpapi.FieldError("attachmentIds", "attachment_not_found", "One of the files is no longer available. Upload it again.")
		}
		if err != nil {
			return err
		}
		purchaseID, err = LinkAttachment(ctx, transaction, householdID, itemID, purchaseID, attachmentID, kind)
		if err != nil {
			return err
		}
	}
	return nil
}

// LinkAttachment links one attachment; used by item saves and by the
// attachments module when files are added later. It returns the item's
// purchase, which it creates for the first invoice of an item without one,
// so callers linking several files must pass the returned ID on.
func LinkAttachment(ctx context.Context, transaction *sql.Tx, householdID int64, itemID int64, purchaseID int64, attachmentID int64, kind string) (int64, error) {
	var count int
	transaction.QueryRowContext(ctx, `SELECT count(*) FROM item_attachments WHERE item_id = ?`, itemID).Scan(&count)
	if count >= config.AttachmentsPerItemMaximum {
		return purchaseID, httpapi.NewError(400, "too_many_attachments", fmt.Sprintf("An item can have up to %d files.", config.AttachmentsPerItemMaximum))
	}
	if kind == "invoice" {
		if purchaseID == 0 {
			result, err := transaction.ExecContext(ctx, `INSERT INTO purchases (household_id, created_at) VALUES (?, ?)`, householdID, store.Now())
			if err != nil {
				return 0, err
			}
			purchaseID, _ = result.LastInsertId()
			if _, err := transaction.ExecContext(ctx, `UPDATE items SET purchase_id = ? WHERE id = ?`, purchaseID, itemID); err != nil {
				return 0, err
			}
		}
		_, err := transaction.ExecContext(ctx, `INSERT OR IGNORE INTO purchase_attachments (purchase_id, attachment_id) VALUES (?, ?)`, purchaseID, attachmentID)
		return purchaseID, err
	}
	_, err := transaction.ExecContext(ctx, `INSERT OR IGNORE INTO item_attachments (item_id, attachment_id, position) VALUES (?, ?, ?)`, itemID, attachmentID, count)
	if err != nil || kind != "photo" {
		return purchaseID, err
	}
	_, err = transaction.ExecContext(ctx, `UPDATE items SET primary_photo_id = ? WHERE id = ? AND primary_photo_id IS NULL`, attachmentID, itemID)
	return purchaseID, err
}

func (s *Service) previousResult(ctx context.Context, userID int64, key string) (int64, bool) {
	if key == "" {
		return 0, false
	}
	var itemID int64
	err := s.store.Database.QueryRowContext(ctx, `SELECT resource_id FROM idempotency_keys WHERE user_id = ? AND idempotency_key = ?`, userID, key).Scan(&itemID)
	return itemID, err == nil
}

func rememberResult(ctx context.Context, transaction *sql.Tx, userID int64, key string, itemID int64) error {
	if key == "" {
		return nil
	}
	_, err := transaction.ExecContext(ctx, `INSERT INTO idempotency_keys (user_id, idempotency_key, resource_id, created_at) VALUES (?, ?, ?, ?)`, userID, key, itemID, store.Now())
	return err
}

// possibleDuplicates finds earlier items with the same source link, or the
// same name and seller. It only warns; repeat purchases are legitimate.
func (s *Service) possibleDuplicates(ctx context.Context, householdID int64, item Item) ([]Reference, error) {
	return loadReferences(ctx, s.store.Database, `SELECT i.id, i.name FROM items i LEFT JOIN purchases p ON p.id = i.purchase_id
		WHERE i.household_id = ? AND i.id != ? AND i.deleted_at IS NULL
		AND ((i.source_url != '' AND i.source_url = ?) OR (lower(i.name) = lower(?) AND coalesce(p.seller, '') = ?))
		ORDER BY i.created_at DESC LIMIT 3`, householdID, item.ID, item.SourceURL, item.Name, item.Seller)
}

type ListQuery struct {
	Text         string
	RoomID       int64
	CategoryID   int64
	TagID        int64
	CollectionID int64
	Unassigned   bool
	HasReceipt   bool
	PriceKnown   bool
	Condition    string
	InWarranty   bool
	Deleted      bool
	Sort         string
	Cursor       string
	Limit        int
}

func (s *Service) List(ctx context.Context, householdID int64, query ListQuery) (Page, error) {
	conditions := []string{"i.household_id = ?"}
	arguments := []any{householdID}
	add := func(condition string, values ...any) {
		conditions = append(conditions, condition)
		arguments = append(arguments, values...)
	}
	if query.Deleted {
		add("i.deleted_at IS NOT NULL")
	} else {
		add("i.deleted_at IS NULL")
	}
	if expression := searchExpression(query.Text); expression != "" {
		add("i.id IN (SELECT rowid FROM items_search WHERE items_search MATCH ?)", expression)
	}
	if query.RoomID != 0 {
		add("i.room_id = ?", query.RoomID)
	}
	if query.Unassigned {
		add("i.room_id IS NULL")
	}
	if query.CategoryID != 0 {
		add("i.category_id = ?", query.CategoryID)
	}
	if query.TagID != 0 {
		add("i.id IN (SELECT item_id FROM item_tags WHERE tag_id = ?)", query.TagID)
	}
	if query.CollectionID != 0 {
		add("i.id IN (SELECT item_id FROM item_collections WHERE collection_id = ?)", query.CollectionID)
	}
	if query.HasReceipt {
		add("i.purchase_id IN (SELECT purchase_id FROM purchase_attachments)")
	}
	if query.Condition != "" {
		add("i.condition = ?", query.Condition)
	}
	if query.InWarranty {
		add("i.warranty_until >= date('now')")
	}
	if query.PriceKnown {
		add("i.paid_unit_minor IS NOT NULL")
	}
	where := " WHERE " + strings.Join(conditions, " AND ")

	var total int
	if err := s.store.Database.QueryRowContext(ctx, `SELECT count(*) FROM items i`+where, arguments...).Scan(&total); err != nil {
		return Page{}, err
	}

	order := map[string]string{
		sortNewest:   "i.created_at DESC, i.id DESC",
		sortPurchase: "p.purchased_on IS NULL, p.purchased_on DESC, i.id DESC",
		sortName:     "i.name COLLATE NOCASE, i.id",
	}[query.Sort]
	if order == "" {
		order = "i.created_at DESC, i.id DESC"
	}
	if query.Deleted {
		order = "i.deleted_at DESC"
	}
	limit := query.Limit
	if limit <= 0 || limit > config.PageSizeMaximum {
		limit = config.PageSizeDefault
	}
	offset := decodeCursor(query.Cursor)

	// ponytail: offset pagination; fine for household-sized lists, switch to keyset if lists reach 100k items.
	rows, err := s.store.Database.QueryContext(ctx, `SELECT i.id, i.name, i.paid_unit_minor, coalesce(i.paid_currency, ''),
			coalesce(p.purchased_on, ''), coalesce(p.purchased_precision, ''), coalesce(r.id, 0), coalesce(r.name, ''),
			coalesce(i.primary_photo_id, 0), i.purchase_id IN (SELECT purchase_id FROM purchase_attachments), i.created_at, coalesce(i.deleted_at, '')
		`+itemJoins+where+` ORDER BY `+order+` LIMIT ? OFFSET ?`, append(arguments, limit, offset)...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	page := Page{Items: []Summary{}, Total: total}
	for rows.Next() {
		var summary Summary
		var paidMinor sql.NullInt64
		var currency, purchasedOn, precision, roomName string
		var roomID int64
		var hasReceipt sql.NullBool
		if err := rows.Scan(&summary.ID, &summary.Name, &paidMinor, &currency, &purchasedOn, &precision, &roomID, &roomName,
			&summary.PrimaryPhotoID, &hasReceipt, &summary.CreatedAt, &summary.DeletedAt); err != nil {
			return Page{}, err
		}
		summary.PricePaid = moneyFrom(paidMinor, currency)
		summary.PurchaseDate = dateFrom(purchasedOn, precision)
		summary.Room = referenceFrom(roomID, roomName)
		summary.HasReceipt = hasReceipt.Bool
		page.Items = append(page.Items, summary)
	}
	if offset+len(page.Items) < total {
		page.NextCursor = encodeCursor(offset + len(page.Items))
	}
	return page, rows.Err()
}

func (s *Service) Recent(ctx context.Context, householdID int64) ([]Summary, error) {
	page, err := s.List(ctx, householdID, ListQuery{Sort: sortNewest, Limit: config.RecentlyAddedCount})
	return page.Items, err
}

// Trash moves an item to Recently deleted; Restore brings it back.
func (s *Service) Trash(ctx context.Context, householdID int64, itemID int64) error {
	return s.setDeleted(ctx, householdID, itemID, true)
}

func (s *Service) Restore(ctx context.Context, householdID int64, itemID int64) error {
	return s.setDeleted(ctx, householdID, itemID, false)
}

func (s *Service) setDeleted(ctx context.Context, householdID int64, itemID int64, deleted bool) error {
	return s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		deletedAt := sql.NullString{String: store.Now(), Valid: deleted}
		result, err := transaction.ExecContext(ctx, `UPDATE items SET deleted_at = ?, revision = revision + 1 WHERE id = ? AND household_id = ? AND (deleted_at IS NULL) = ?`,
			deletedAt, itemID, householdID, deleted)
		if err != nil {
			return err
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return ErrNotFound
		}
		return Reindex(ctx, transaction, itemID)
	})
}

// Purge permanently deletes a trashed item. Files no longer referenced by
// any other item or purchase are removed later by the blob reconciler.
func (s *Service) Purge(ctx context.Context, householdID int64, userID int64, itemID int64) error {
	return s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		var purchaseID sql.NullInt64
		err := transaction.QueryRowContext(ctx, `SELECT purchase_id FROM items WHERE id = ? AND household_id = ? AND deleted_at IS NOT NULL`, itemID, householdID).Scan(&purchaseID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if _, err := transaction.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, itemID); err != nil {
			return err
		}
		if _, err := transaction.ExecContext(ctx, `DELETE FROM items_search WHERE rowid = ?`, itemID); err != nil {
			return err
		}
		if purchaseID.Valid {
			if _, err := transaction.ExecContext(ctx, `DELETE FROM purchases WHERE id = ? AND NOT EXISTS (SELECT 1 FROM items WHERE purchase_id = ?)`, purchaseID.Int64, purchaseID.Int64); err != nil {
				return err
			}
		}
		if err := deleteUnlinkedAttachments(ctx, transaction, householdID); err != nil {
			return err
		}
		return audit.Record(ctx, transaction, householdID, userID, audit.ItemPurged, "")
	})
}

// deleteUnlinkedAttachments removes attachment rows no longer linked to
// anything. Their files become unreferenced and are reclaimed after the grace
// period, so a running backup never loses them mid-archive.
func deleteUnlinkedAttachments(ctx context.Context, transaction *sql.Tx, householdID int64) error {
	_, err := transaction.ExecContext(ctx, `DELETE FROM attachments WHERE household_id = ?
		AND id NOT IN (SELECT attachment_id FROM item_attachments)
		AND id NOT IN (SELECT attachment_id FROM purchase_attachments)
		AND id NOT IN (SELECT attachment_id FROM import_images)
		AND id NOT IN (SELECT photo_id FROM rooms WHERE photo_id IS NOT NULL)
		AND created_at < ?`, householdID, store.FormatTime(time.Now().Add(-config.OrphanBlobGracePeriod)))
	return err
}

func (s *Service) History(ctx context.Context, householdID int64, itemID int64) ([]Revision, error) {
	if _, err := s.Get(ctx, householdID, itemID); err != nil {
		return nil, err
	}
	rows, err := s.store.Database.QueryContext(ctx, `SELECT source, changes_json, changed_at FROM item_revisions WHERE item_id = ? ORDER BY changed_at DESC, id DESC`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	revisions := []Revision{}
	for rows.Next() {
		var revision Revision
		var encoded string
		if err := rows.Scan(&revision.Source, &encoded, &revision.ChangedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(encoded), &revision.Changes)
		revisions = append(revisions, revision)
	}
	return revisions, rows.Err()
}

// Totals returns paid totals per currency; currencies are never summed together.
func (s *Service) Totals(ctx context.Context, householdID int64) (map[string]string, error) {
	rows, err := s.store.Database.QueryContext(ctx, `SELECT paid_currency, sum(paid_unit_minor * quantity) FROM items
		WHERE household_id = ? AND deleted_at IS NULL AND paid_unit_minor IS NOT NULL GROUP BY paid_currency`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	totals := map[string]string{}
	for rows.Next() {
		var currency string
		var minor int64
		if err := rows.Scan(&currency, &minor); err != nil {
			return nil, err
		}
		totals[currency] = money.FormatDecimal(minor, currency)
	}
	return totals, rows.Err()
}

// PurgeAbandonedAttachments is called periodically to clear uploads that
// were never attached to an item.
func (s *Service) PurgeAbandonedAttachments(ctx context.Context) error {
	return s.store.Transaction(ctx, func(transaction *sql.Tx) error {
		_, err := transaction.ExecContext(ctx, `DELETE FROM attachments
			WHERE id NOT IN (SELECT attachment_id FROM item_attachments)
			AND id NOT IN (SELECT attachment_id FROM purchase_attachments)
			AND id NOT IN (SELECT attachment_id FROM import_images)
		AND id NOT IN (SELECT photo_id FROM rooms WHERE photo_id IS NOT NULL)
			AND created_at < ?`, store.FormatTime(time.Now().Add(-config.OrphanBlobGracePeriod)))
		return err
	})
}
