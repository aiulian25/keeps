-- Page data sent by the "Add to Keeps" browser button, rebuilt as a minimal
-- HTML document. When present, the worker extracts from it instead of
-- fetching the shop page.
ALTER TABLE imports ADD COLUMN provided_document TEXT NOT NULL DEFAULT '';
