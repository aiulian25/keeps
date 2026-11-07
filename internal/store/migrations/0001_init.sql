-- Keeps initial schema. Timestamps are RFC 3339 UTC text. Money is integer
-- minor units plus an ISO 4217 currency; never floating point.

CREATE TABLE households (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'GBP' CHECK (length(currency) = 3),
    created_at TEXT NOT NULL
);

CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households(id),
    username TEXT NOT NULL UNIQUE COLLATE NOCASE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    webauthn_handle BLOB NOT NULL UNIQUE,
    is_admin INTEGER NOT NULL DEFAULT 0,
    theme TEXT NOT NULL DEFAULT 'dark' CHECK (theme IN ('dark', 'light')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE bootstrap_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    claimed INTEGER NOT NULL DEFAULT 0,
    code_verifier BLOB,
    code_expires_at TEXT
);

CREATE TABLE sessions (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BLOB NOT NULL UNIQUE,
    csrf_token TEXT NOT NULL,
    awaiting_second_factor INTEGER NOT NULL DEFAULT 0,
    user_agent TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    reauthenticated_at TEXT
);
CREATE INDEX sessions_user ON sessions(user_id);

CREATE TABLE login_failures (
    subject TEXT PRIMARY KEY,
    failures INTEGER NOT NULL,
    blocked_until TEXT NOT NULL
);

CREATE TABLE totp_credentials (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    encrypted_secret BLOB NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 0,
    last_used_step INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE TABLE recovery_codes (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash BLOB NOT NULL,
    used_at TEXT
);

CREATE TABLE passkeys (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id BLOB NOT NULL UNIQUE,
    credential_json TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL,
    last_used_at TEXT
);

CREATE TABLE webauthn_challenges (
    id TEXT PRIMARY KEY,
    user_id INTEGER,
    ceremony TEXT NOT NULL CHECK (ceremony IN ('registration', 'login')),
    session_json TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE TABLE recovery_capabilities (
    id INTEGER PRIMARY KEY,
    code_hash BLOB NOT NULL,
    expires_at TEXT NOT NULL,
    used_at TEXT
);

CREATE TABLE rooms (
    id INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households(id),
    name TEXT NOT NULL COLLATE NOCASE,
    parent_id INTEGER REFERENCES rooms(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL,
    UNIQUE (household_id, name)
);

CREATE TABLE categories (
    id INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households(id),
    name TEXT NOT NULL COLLATE NOCASE,
    created_at TEXT NOT NULL,
    UNIQUE (household_id, name)
);

CREATE TABLE tags (
    id INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households(id),
    name TEXT NOT NULL COLLATE NOCASE,
    created_at TEXT NOT NULL,
    UNIQUE (household_id, name)
);

CREATE TABLE collections (
    id INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households(id),
    name TEXT NOT NULL COLLATE NOCASE,
    created_at TEXT NOT NULL,
    UNIQUE (household_id, name)
);

CREATE TABLE attachments (
    id INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households(id),
    kind TEXT NOT NULL CHECK (kind IN ('photo', 'invoice', 'document')),
    storage_key TEXT NOT NULL UNIQUE,
    preview_key TEXT,
    thumbnail_key TEXT,
    filename TEXT NOT NULL,
    media_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
    sha256 TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE purchases (
    id INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households(id),
    seller TEXT NOT NULL DEFAULT '',
    order_reference TEXT NOT NULL DEFAULT '',
    purchased_on TEXT,
    purchased_precision TEXT CHECK (purchased_precision IN ('date', 'datetime')),
    created_at TEXT NOT NULL,
    CHECK ((purchased_on IS NULL) = (purchased_precision IS NULL))
);

CREATE TABLE items (
    id INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households(id),
    name TEXT NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 200),
    description TEXT NOT NULL DEFAULT '',
    quantity INTEGER NOT NULL DEFAULT 1 CHECK (quantity >= 1),
    brand TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    serial_number TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    room_id INTEGER REFERENCES rooms(id) ON DELETE SET NULL,
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    purchase_id INTEGER REFERENCES purchases(id) ON DELETE SET NULL,
    paid_unit_minor INTEGER CHECK (paid_unit_minor IS NULL OR paid_unit_minor >= 0),
    paid_currency TEXT CHECK (paid_currency IS NULL OR length(paid_currency) = 3),
    primary_photo_id INTEGER REFERENCES attachments(id) ON DELETE SET NULL,
    source_url TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    CHECK ((paid_unit_minor IS NULL) = (paid_currency IS NULL))
);
CREATE INDEX items_household_created ON items(household_id, deleted_at, created_at);
CREATE INDEX items_room ON items(room_id);
CREATE INDEX items_purchase ON items(purchase_id);

CREATE TABLE item_attachments (
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    attachment_id INTEGER NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (item_id, attachment_id)
);

CREATE TABLE purchase_attachments (
    purchase_id INTEGER NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
    attachment_id INTEGER NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
    PRIMARY KEY (purchase_id, attachment_id)
);

CREATE TABLE item_tags (
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (item_id, tag_id)
);

CREATE TABLE item_collections (
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    collection_id INTEGER NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    PRIMARY KEY (item_id, collection_id)
);

CREATE TABLE listing_snapshots (
    id INTEGER PRIMARY KEY,
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    source_url TEXT NOT NULL,
    source_host TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    price_minor INTEGER,
    currency TEXT,
    retrieved_at TEXT NOT NULL,
    extractor_version TEXT NOT NULL,
    CHECK ((price_minor IS NULL) = (currency IS NULL))
);
CREATE INDEX listing_snapshots_item ON listing_snapshots(item_id);

CREATE TABLE field_provenance (
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    field TEXT NOT NULL,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('user', 'import')),
    snapshot_id INTEGER REFERENCES listing_snapshots(id) ON DELETE SET NULL,
    recorded_at TEXT NOT NULL,
    PRIMARY KEY (item_id, field)
);

CREATE TABLE imports (
    id TEXT PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households(id),
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_url TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('queued', 'running', 'done', 'failed', 'cancelled', 'saved')),
    attempts INTEGER NOT NULL DEFAULT 0,
    lease_expires_at TEXT,
    error_code TEXT NOT NULL DEFAULT '',
    result_json TEXT NOT NULL DEFAULT '',
    retrieved_at TEXT,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX imports_state ON imports(state, created_at);

CREATE TABLE import_images (
    import_id TEXT NOT NULL REFERENCES imports(id) ON DELETE CASCADE,
    attachment_id INTEGER NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    PRIMARY KEY (import_id, attachment_id)
);

CREATE TABLE item_revisions (
    id INTEGER PRIMARY KEY,
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    source TEXT NOT NULL CHECK (source IN ('user', 'import', 'restore')),
    changes_json TEXT NOT NULL,
    changed_at TEXT NOT NULL
);
CREATE INDEX item_revisions_item ON item_revisions(item_id, changed_at);

CREATE TABLE audit_events (
    id INTEGER PRIMARY KEY,
    household_id INTEGER,
    user_id INTEGER,
    kind TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE idempotency_keys (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL,
    resource_id INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (user_id, idempotency_key)
);

CREATE VIRTUAL TABLE items_search USING fts5(
    name, description, brand_model, seller, notes, groups,
    tokenize = 'unicode61 remove_diacritics 2',
    prefix = '2 3'
);

INSERT INTO bootstrap_state (id, claimed) VALUES (1, 0);
