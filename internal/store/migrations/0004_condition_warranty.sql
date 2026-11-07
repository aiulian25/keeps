-- Optional physical condition and warranty end date for items.
ALTER TABLE items ADD COLUMN condition TEXT NOT NULL DEFAULT ''
    CHECK (condition IN ('', 'new', 'like_new', 'good', 'fair', 'poor', 'needs_repair'));
ALTER TABLE items ADD COLUMN warranty_until TEXT;
