package items

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/money"
	"github.com/aiulian25/keeps/internal/organise"
	"github.com/aiulian25/keeps/internal/store"
)

// querier is satisfied by *sql.DB and *sql.Tx.
type querier interface {
	QueryContext(ctx context.Context, query string, arguments ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, arguments ...any) *sql.Row
	ExecContext(ctx context.Context, query string, arguments ...any) (sql.Result, error)
}

var (
	errItemNotFound     = errors.New("item not found")
	errPurchaseNotFound = httpapi.NewError(http.StatusNotFound, "purchase_not_found", "That purchase no longer exists. Save the item without it, or pick it again.")
)

const itemColumns = `i.id, i.name, i.description, i.quantity, i.brand, i.model, i.serial_number, i.notes,
	coalesce(p.seller, ''), coalesce(p.order_reference, ''), i.paid_unit_minor, coalesce(i.paid_currency, ''),
	coalesce(p.purchased_on, ''), coalesce(p.purchased_precision, ''), coalesce(i.purchase_id, 0),
	coalesce(r.id, 0), coalesce(r.name, ''), coalesce(c.id, 0), coalesce(c.name, ''),
	coalesce(i.primary_photo_id, 0), i.source_url, i.revision, i.created_at, i.updated_at, coalesce(i.deleted_at, ''),
	i.condition, coalesce(i.warranty_until, ''), i.measurements_json`

const itemJoins = ` FROM items i
	LEFT JOIN purchases p ON p.id = i.purchase_id
	LEFT JOIN rooms r ON r.id = i.room_id
	LEFT JOIN categories c ON c.id = i.category_id`

func moneyFrom(minor sql.NullInt64, currency string) *Money {
	if !minor.Valid {
		return nil
	}
	return &Money{Amount: money.FormatDecimal(minor.Int64, currency), Currency: currency}
}

func dateFrom(value string, precision string) *PurchaseDate {
	if value == "" {
		return nil
	}
	return &PurchaseDate{Value: value, Precision: precision}
}

func referenceFrom(identifier int64, name string) *Reference {
	if identifier == 0 {
		return nil
	}
	return &Reference{ID: identifier, Name: name}
}

func loadItem(ctx context.Context, database querier, householdID int64, itemID int64) (Item, error) {
	var item Item
	var paidMinor sql.NullInt64
	var paidCurrency, purchasedOn, precision, roomName, categoryName, measurements string
	var roomID, categoryID int64
	err := database.QueryRowContext(ctx, `SELECT `+itemColumns+itemJoins+` WHERE i.id = ? AND i.household_id = ?`, itemID, householdID).Scan(
		&item.ID, &item.Name, &item.Description, &item.Quantity, &item.Brand, &item.Model, &item.SerialNumber, &item.Notes,
		&item.Seller, &item.OrderReference, &paidMinor, &paidCurrency, &purchasedOn, &precision, &item.PurchaseID,
		&roomID, &roomName, &categoryID, &categoryName, &item.PrimaryPhotoID, &item.SourceURL, &item.Revision,
		&item.CreatedAt, &item.UpdatedAt, &item.DeletedAt, &item.Condition, &item.WarrantyUntil, &measurements)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, errItemNotFound
	}
	if err != nil {
		return Item{}, err
	}
	item.Measurements = decodeMeasurements(measurements)
	item.PricePaid = moneyFrom(paidMinor, paidCurrency)
	item.PurchaseDate = dateFrom(purchasedOn, precision)
	item.Room = referenceFrom(roomID, roomName)
	item.Category = referenceFrom(categoryID, categoryName)

	if item.Tags, err = loadReferences(ctx, database, `SELECT t.id, t.name FROM tags t JOIN item_tags x ON x.tag_id = t.id WHERE x.item_id = ? ORDER BY t.name`, itemID); err != nil {
		return Item{}, err
	}
	if item.Collections, err = loadReferences(ctx, database, `SELECT c.id, c.name FROM collections c JOIN item_collections x ON x.collection_id = c.id WHERE x.item_id = ? ORDER BY c.name`, itemID); err != nil {
		return Item{}, err
	}
	if item.SharedWith, err = loadReferences(ctx, database, `SELECT id, name FROM items WHERE purchase_id = ? AND id != ? AND purchase_id IS NOT NULL AND deleted_at IS NULL ORDER BY name`, item.PurchaseID, itemID); err != nil {
		return Item{}, err
	}
	if err := loadAttachments(ctx, database, &item); err != nil {
		return Item{}, err
	}
	if item.Listing, err = loadListing(ctx, database, itemID); err != nil {
		return Item{}, err
	}
	if item.Parts, err = loadParts(ctx, database, itemID); err != nil {
		return Item{}, err
	}
	if item.Units, err = loadUnits(ctx, database, itemID); err != nil {
		return Item{}, err
	}
	item.FieldSources, err = loadProvenance(ctx, database, itemID)
	return item, err
}

func loadReferences(ctx context.Context, database querier, query string, arguments ...any) ([]Reference, error) {
	rows, err := database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	references := []Reference{}
	for rows.Next() {
		var reference Reference
		if err := rows.Scan(&reference.ID, &reference.Name); err != nil {
			return nil, err
		}
		references = append(references, reference)
	}
	return references, rows.Err()
}

// loadAttachments returns the item's photos and documents, including
// invoices attached to a purchase shared with other items.
func loadAttachments(ctx context.Context, database querier, item *Item) error {
	rows, err := database.QueryContext(ctx, `SELECT a.id, a.kind, a.filename, a.media_type, a.size_bytes, a.created_at, a.preview_key IS NOT NULL, 0, x.position
			FROM attachments a JOIN item_attachments x ON x.attachment_id = a.id WHERE x.item_id = ?
		UNION ALL
		SELECT a.id, a.kind, a.filename, a.media_type, a.size_bytes, a.created_at, a.preview_key IS NOT NULL,
			(SELECT count(*) FROM items o WHERE o.purchase_id = pa.purchase_id AND o.deleted_at IS NULL) > 1, 0
			FROM attachments a JOIN purchase_attachments pa ON pa.attachment_id = a.id WHERE pa.purchase_id = ?
		ORDER BY 9, 6`, item.ID, item.PurchaseID)
	if err != nil {
		return err
	}
	defer rows.Close()
	item.Photos, item.Documents = []Attachment{}, []Attachment{}
	for rows.Next() {
		var attachment Attachment
		var position int
		if err := rows.Scan(&attachment.ID, &attachment.Kind, &attachment.Filename, &attachment.MediaType, &attachment.SizeBytes,
			&attachment.CreatedAt, &attachment.HasPreview, &attachment.Shared, &position); err != nil {
			return err
		}
		if attachment.Kind == "photo" {
			item.Photos = append(item.Photos, attachment)
			continue
		}
		item.Documents = append(item.Documents, attachment)
	}
	return rows.Err()
}

func loadListing(ctx context.Context, database querier, itemID int64) (*Listing, error) {
	var listing Listing
	var price sql.NullInt64
	var currency sql.NullString
	err := database.QueryRowContext(ctx, `SELECT title, price_minor, currency, source_url, source_host, retrieved_at FROM listing_snapshots
		WHERE item_id = ? ORDER BY retrieved_at DESC LIMIT 1`, itemID).Scan(&listing.Title, &price, &currency, &listing.SourceURL, &listing.SourceHost, &listing.RetrievedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	listing.Price = moneyFrom(price, currency.String)
	return &listing, nil
}

func loadProvenance(ctx context.Context, database querier, itemID int64) (map[string]string, error) {
	rows, err := database.QueryContext(ctx, `SELECT field, source_kind FROM field_provenance WHERE item_id = ?`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sources := map[string]string{}
	for rows.Next() {
		var field, source string
		if err := rows.Scan(&field, &source); err != nil {
			return nil, err
		}
		sources[field] = source
	}
	return sources, rows.Err()
}

// writePurchase creates, joins or updates the purchase record for an item,
// and reports whether facts other items may share actually changed.
// Editing an item's own purchase replaces its facts, for every item sharing
// it. Joining another purchase only fills in what the new item states: blank
// fields never clear the facts the purchase already holds.
func writePurchase(ctx context.Context, transaction *sql.Tx, householdID int64, currentPurchaseID int64, values normalised) (int64, bool, error) {
	purchaseID := currentPurchaseID
	joining := values.PurchaseID != 0 && values.PurchaseID != currentPurchaseID
	if values.PurchaseID != 0 {
		var owner int64
		err := transaction.QueryRowContext(ctx, `SELECT household_id FROM purchases WHERE id = ?`, values.PurchaseID).Scan(&owner)
		if err != nil || owner != householdID {
			return 0, false, errPurchaseNotFound
		}
		purchaseID = values.PurchaseID
	}

	if purchaseID == 0 && !values.hasPurchaseDetails() {
		return 0, false, nil
	}
	if purchaseID == 0 {
		date := sql.NullString{String: values.purchasedOn, Valid: values.purchasedOn != ""}
		precision := sql.NullString{String: values.precision, Valid: values.purchasedOn != ""}
		result, err := transaction.ExecContext(ctx, `INSERT INTO purchases (household_id, seller, order_reference, purchased_on, purchased_precision, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			householdID, values.Seller, values.OrderReference, date, precision, store.Now())
		if err != nil {
			return 0, false, err
		}
		created, err := result.LastInsertId()
		return created, false, err
	}

	var stored purchaseRecord
	err := transaction.QueryRowContext(ctx, `SELECT seller, order_reference, coalesce(purchased_on, ''), coalesce(purchased_precision, '') FROM purchases WHERE id = ?`, purchaseID).
		Scan(&stored.seller, &stored.orderReference, &stored.date, &stored.precision)
	if err != nil {
		return 0, false, err
	}
	wanted := purchaseRecord{seller: values.Seller, orderReference: values.OrderReference, date: values.purchasedOn, precision: values.precision}
	if joining {
		wanted = stored.filledFrom(wanted)
	}
	if wanted == stored {
		return purchaseID, false, nil
	}
	_, err = transaction.ExecContext(ctx, `UPDATE purchases SET seller = ?, order_reference = ?, purchased_on = ?, purchased_precision = ? WHERE id = ?`,
		wanted.seller, wanted.orderReference, nullableText(wanted.date), nullableText(wanted.precision), purchaseID)
	return purchaseID, true, err
}

// purchaseRecord holds the facts every item on a purchase shares.
type purchaseRecord struct {
	seller, orderReference, date, precision string
}

// filledFrom keeps each stored fact unless stated says otherwise.
func (stored purchaseRecord) filledFrom(stated purchaseRecord) purchaseRecord {
	result := stored
	if stated.seller != "" {
		result.seller = stated.seller
	}
	if stated.orderReference != "" {
		result.orderReference = stated.orderReference
	}
	if stated.date != "" {
		result.date, result.precision = stated.date, stated.precision
	}
	return result
}

// touchSiblings advances the revision of every other item on a purchase whose
// shared facts changed, so their open forms get a conflict instead of
// silently putting the old facts back.
func touchSiblings(ctx context.Context, transaction *sql.Tx, purchaseID int64, itemID int64) error {
	_, err := transaction.ExecContext(ctx, `UPDATE items SET revision = revision + 1, updated_at = ? WHERE purchase_id = ? AND id != ?`, store.Now(), purchaseID, itemID)
	return err
}

func writeGroups(ctx context.Context, transaction *sql.Tx, householdID int64, itemID int64, values normalised) error {
	for _, link := range []struct {
		kind   organise.Kind
		table  string
		column string
		names  []string
	}{
		{organise.Tags, "item_tags", "tag_id", values.Tags},
		{organise.Collections, "item_collections", "collection_id", values.Collections},
	} {
		if _, err := transaction.ExecContext(ctx, `DELETE FROM `+link.table+` WHERE item_id = ?`, itemID); err != nil {
			return err
		}
		for _, name := range link.names {
			groupID, err := organise.ResolveByName(ctx, transaction, householdID, link.kind, name)
			if err != nil {
				return err
			}
			if _, err := transaction.ExecContext(ctx, `INSERT OR IGNORE INTO `+link.table+` (item_id, `+link.column+`) VALUES (?, ?)`, itemID, groupID); err != nil {
				return err
			}
		}
	}
	return nil
}

func nullableID(identifier int64) sql.NullInt64 {
	return sql.NullInt64{Int64: identifier, Valid: identifier != 0}
}

// Reindex refreshes the full-text index rows for the given items.
func Reindex(ctx context.Context, database querier, itemIDs ...int64) error {
	for _, itemID := range itemIDs {
		if _, err := database.ExecContext(ctx, `DELETE FROM items_search WHERE rowid = ?`, itemID); err != nil {
			return err
		}
		_, err := database.ExecContext(ctx, `INSERT INTO items_search (rowid, name, description, brand_model, seller, notes, groups)
			SELECT i.id, i.name, i.description,
				i.brand || ' ' || i.model || ' ' || i.serial_number || ' ' || coalesce((SELECT group_concat(u.label || ' ' || u.serial_number, ' ') FROM item_units u WHERE u.item_id = i.id), ''),
				coalesce(p.seller, ''), i.notes,
				coalesce(r.name, '') || ' ' || coalesce(c.name, '') || ' ' ||
				coalesce((SELECT group_concat(t.name, ' ') FROM tags t JOIN item_tags x ON x.tag_id = t.id WHERE x.item_id = i.id), '') || ' ' ||
				coalesce((SELECT group_concat(k.name, ' ') FROM collections k JOIN item_collections y ON y.collection_id = k.id WHERE y.item_id = i.id), '')
			`+strings.TrimPrefix(itemJoins, " ")+` WHERE i.id = ? AND i.deleted_at IS NULL`, itemID)
		if err != nil {
			return err
		}
	}
	return nil
}

// searchExpression turns free text into a safe FTS5 query: every word becomes
// a quoted prefix term, so user input can never inject FTS syntax.
func searchExpression(text string) string {
	var terms []string
	for _, word := range strings.FieldsFunc(strings.ToLower(text), func(character rune) bool {
		isWordCharacter := character == '\'' || character == '-' || character >= '0' && character <= '9' || character >= 'a' && character <= 'z' || character > 127
		return !isWordCharacter
	}) {
		word = strings.Trim(word, "'-")
		if word == "" {
			continue
		}
		terms = append(terms, `"`+strings.ReplaceAll(word, `"`, ``)+`"*`)
	}
	return strings.Join(terms, " ")
}

func encodeCursor(offset int) string {
	return strconv.Itoa(offset)
}

func decodeCursor(cursor string) int {
	offset, err := strconv.Atoi(cursor)
	if err != nil || offset < 0 {
		return 0
	}
	return offset
}
