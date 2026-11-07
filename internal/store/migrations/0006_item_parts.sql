-- Compatible consumables and replacement parts recorded against an item:
-- what the owner noted, never fetched or checked by Keeps.
CREATE TABLE item_parts (
    id INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households(id),
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    part_number TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX item_parts_item ON item_parts(item_id);
