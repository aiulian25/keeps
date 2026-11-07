-- Named pages in a PDF the owner wants to find again. The PDF itself is
-- never changed; a bookmark is just a page number and a label.
CREATE TABLE document_bookmarks (
    id INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households(id),
    attachment_id INTEGER NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
    page INTEGER NOT NULL CHECK (page >= 1),
    label TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX document_bookmarks_attachment ON document_bookmarks(attachment_id, page);
