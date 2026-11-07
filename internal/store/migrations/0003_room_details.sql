-- Optional description and photo for rooms.
ALTER TABLE rooms ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE rooms ADD COLUMN photo_id INTEGER REFERENCES attachments(id) ON DELETE SET NULL;
