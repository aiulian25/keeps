-- Optional measurements and a fitting note, kept as the person entered them:
-- exact decimal strings with their units, as JSON ('' when none).
ALTER TABLE items ADD COLUMN measurements_json TEXT NOT NULL DEFAULT '';
