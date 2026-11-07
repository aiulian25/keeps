import { useState, type ReactNode } from "react";
import { Download, Eye, EyeOff, FileText } from "lucide-react";
import { attachmentUrl } from "../../api/client";
import type { Attachment } from "../../api/types";
import { Card } from "../../components/Bits";
import { buttonClass } from "../../components/Button";
import { COMMON } from "../../constants/copy";
import { DocumentViewer } from "./DocumentViewer";
import { RedactionDialog } from "./RedactionDialog";
import { DOCUMENTS_COPY } from "./documentsCopy";
import { fileSummary, PDF_MEDIA_TYPE } from "./fileFormat";
import styles from "./Documents.module.css";

function FileBadge({ attachment }: { attachment: Attachment }) {
  if (attachment.mediaType === PDF_MEDIA_TYPE) {
    return (
      <span className={styles.pdfBadge} aria-hidden="true">
        {DOCUMENTS_COPY.pdfBadge}
      </span>
    );
  }
  if (attachment.hasPreview) {
    return (
      <img
        className={styles.thumbnail}
        src={attachmentUrl(attachment.id, "thumbnail")}
        alt=""
        loading="lazy"
      />
    );
  }
  return (
    <span className={styles.fileBadge} aria-hidden="true">
      <FileText />
    </span>
  );
}

interface FileRowProps {
  attachment: Attachment;
  /** Extra line under the summary, e.g. the item a document belongs to. */
  detail?: ReactNode;
  /** Overflow menu or other per-file actions. */
  actions?: ReactNode;
}

/** Panel 10 file row: type badge, name, summary, Download. */
export function FileRow({ attachment, detail, actions }: FileRowProps) {
  const [viewing, setViewing] = useState(false);
  const [sharing, setSharing] = useState(false);
  return (
    <Card className={styles.row}>
      <FileBadge attachment={attachment} />
      <div className={styles.rowText}>
        <button
          type="button"
          className={styles.filenameButton}
          title={attachment.filename}
          onClick={() => setViewing(true)}
        >
          {attachment.filename}
        </button>
        <span className="muted small">{fileSummary(attachment)}</span>
        {detail}
      </div>
      <button
        type="button"
        className={buttonClass("secondary", false, true)}
        onClick={() => setViewing(true)}
        aria-label={DOCUMENTS_COPY.view(attachment.filename)}
      >
        <Eye aria-hidden="true" />
        <span className={styles.downloadText}>{DOCUMENTS_COPY.viewShort}</span>
      </button>
      <a
        href={attachmentUrl(attachment.id)}
        download={attachment.filename}
        className={buttonClass("secondary", false, true)}
        aria-label={DOCUMENTS_COPY.download(attachment.filename)}
      >
        <Download aria-hidden="true" />
        <span className={styles.downloadText}>{COMMON.download}</span>
      </a>
      <button
        type="button"
        className={buttonClass("secondary", false, true)}
        onClick={() => setSharing(true)}
        aria-label={DOCUMENTS_COPY.sharingCopy(attachment.filename)}
        title={DOCUMENTS_COPY.sharingCopyShort}
      >
        <EyeOff aria-hidden="true" />
        <span className={styles.downloadText}>
          {DOCUMENTS_COPY.sharingCopyShort}
        </span>
      </button>
      {actions}
      {sharing && (
        <RedactionDialog
          attachment={attachment}
          onClose={() => setSharing(false)}
        />
      )}
      {viewing && (
        <DocumentViewer
          attachment={attachment}
          onClose={() => setViewing(false)}
        />
      )}
    </Card>
  );
}
