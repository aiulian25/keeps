import type { MissingFile } from "../../api/types";
import { MISSING_COPY } from "./copy";
import styles from "./Data.module.css";

/** Names every file a backup lacks, with the items it belonged to. */
export function MissingFilesList({ files }: { files: MissingFile[] }) {
  return (
    <ul className={styles.missingList} aria-label={MISSING_COPY.label}>
      {files.map((file, index) => (
        <li key={`${file.filename}-${file.copy}-${index}`}>
          {MISSING_COPY.line(
            file.filename,
            MISSING_COPY.copyName[file.copy],
            file.items.length > 0
              ? file.items.join(", ")
              : MISSING_COPY.notLinked,
          )}
        </li>
      ))}
    </ul>
  );
}
