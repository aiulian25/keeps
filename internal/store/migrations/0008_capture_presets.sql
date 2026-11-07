-- Reusable starting details for adding similar belongings. Only organising
-- details and generic notes: never serial numbers, prices, purchases or files.
-- Groups are kept by name, so a preset survives a renamed or deleted group.
CREATE TABLE capture_presets (
    id INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households(id),
    name TEXT NOT NULL COLLATE NOCASE,
    room TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    tags_json TEXT NOT NULL DEFAULT '[]',
    collections_json TEXT NOT NULL DEFAULT '[]',
    condition TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    UNIQUE (household_id, name)
);
