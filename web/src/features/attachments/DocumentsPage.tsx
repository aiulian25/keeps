import { useState } from "react";
import { Search } from "lucide-react";
import { Link } from "react-router";
import { LoadingLine } from "../../components/Bits";
import { Button } from "../../components/Button";
import { FieldShell, FormError } from "../../components/form/Field";
import fieldStyles from "../../components/form/Field.module.css";
import { PageHeader } from "../../components/shell/PageHeader";
import { ROUTES } from "../../constants/routes";
import { DOCUMENTS_COPY } from "./documentsCopy";
import { FileRow } from "./FileRow";
import { useDocumentSearch } from "./useDocuments";
import styles from "./Documents.module.css";

/** Inferred companion to panel 10: every invoice and manual, searchable. */
export function DocumentsPage() {
  const [query, setQuery] = useState("");
  const { documents, total, hasMore, loadMore, loadingMore, moreError, error } =
    useDocumentSearch(query);
  const trimmedQuery = query.trim();

  return (
    <div className={styles.page}>
      <PageHeader title={DOCUMENTS_COPY.title} />
      <FieldShell
        label={DOCUMENTS_COPY.searchLabel}
        className={styles.searchField}
      >
        {({ id, describedBy }) => (
          <div className={fieldStyles.control}>
            <span className={fieldStyles.prefix}>
              <Search aria-hidden="true" />
            </span>
            <input
              id={id}
              className={fieldStyles.input}
              type="search"
              autoComplete="off"
              placeholder={DOCUMENTS_COPY.searchPlaceholder}
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              aria-describedby={describedBy}
            />
          </div>
        )}
      </FieldShell>
      <FormError message={error} />
      {documents === null && !error && <LoadingLine />}
      {documents?.length === 0 && (
        <p className="muted">
          {trimmedQuery
            ? DOCUMENTS_COPY.noMatches(trimmedQuery)
            : DOCUMENTS_COPY.empty}
        </p>
      )}
      {documents && documents.length > 0 && (
        <ul className={styles.list} aria-live="polite">
          {documents.map((entry) => (
            <li key={`${entry.id}-${entry.itemId}`}>
              <FileRow
                attachment={entry}
                detail={
                  <Link
                    to={ROUTES.item(entry.itemId)}
                    className={styles.itemLink}
                  >
                    {entry.itemName}
                  </Link>
                }
              />
            </li>
          ))}
        </ul>
      )}
      {documents && hasMore && (
        <div className={styles.loadMore}>
          <p className="muted small">
            {DOCUMENTS_COPY.showing(documents.length, total)}
          </p>
          <FormError message={moreError} />
          <Button busy={loadingMore} onClick={loadMore}>
            {moreError ? DOCUMENTS_COPY.retryMore : DOCUMENTS_COPY.loadMore}
          </Button>
        </div>
      )}
    </div>
  );
}
