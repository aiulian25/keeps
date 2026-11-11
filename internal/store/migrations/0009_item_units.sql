-- Individual units of an item bought as several, such as two matching
-- speakers: each unit's printed serial and a label telling them apart. The
-- item keeps its quantity, unit price, purchase and receipts.
CREATE TABLE item_units (
    id INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households(id),
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    label TEXT NOT NULL DEFAULT '',
    serial_number TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX item_units_item ON item_units(item_id);
