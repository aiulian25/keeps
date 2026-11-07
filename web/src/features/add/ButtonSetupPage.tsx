import { useEffect, useRef, useState } from "react";
import { Bookmark, Copy } from "lucide-react";
import { Button } from "../../components/Button";
import { Card } from "../../components/Bits";
import { PageHeader } from "../../components/shell/PageHeader";
import { ROUTES } from "../../constants/routes";
import { bookmarkletUrl } from "./bookmarklet";
import { BUTTON_COPY } from "./copy";
import styles from "./Add.module.css";

const COPIED_MESSAGE_MS = 2000;

/** Explains and provides the Add to Keeps bookmark button. */
export function ButtonSetupPage() {
  const link = useRef<HTMLAnchorElement>(null);
  const [copied, setCopied] = useState(false);
  const code = bookmarkletUrl(window.location.origin);

  // React refuses javascript: links in JSX, so the bookmark address is set
  // directly on the element; it is never clicked inside Keeps.
  useEffect(() => {
    link.current?.setAttribute("href", code);
  }, [code]);

  const copy = async () => {
    await navigator.clipboard.writeText(code);
    setCopied(true);
    window.setTimeout(() => setCopied(false), COPIED_MESSAGE_MS);
  };

  return (
    <div className={`stack-large ${styles.buttonSetup}`}>
      <PageHeader title={BUTTON_COPY.title} backTo={ROUTES.add} />
      <p className="muted">{BUTTON_COPY.intro}</p>
      <Card className={styles.buttonCard}>
        <h2>{BUTTON_COPY.stepsLabel}</h2>
        <ol className={styles.buttonSteps}>
          <li>{BUTTON_COPY.stepShowBar}</li>
          <li>
            {BUTTON_COPY.stepDrag}{" "}
            {/* This link exists to be dragged to the bookmarks bar; clicking it inside Keeps does nothing. */}
            {/* eslint-disable-next-line jsx-a11y/anchor-is-valid, jsx-a11y/click-events-have-key-events, jsx-a11y/no-static-element-interactions */}
            <a
              ref={link}
              className={styles.bookmarkButton}
              title={BUTTON_COPY.dragHint}
              onClick={(event) => event.preventDefault()}
            >
              <Bookmark aria-hidden="true" />
              {BUTTON_COPY.buttonLabel}
            </a>
          </li>
          <li>{BUTTON_COPY.stepUse}</li>
        </ol>
        <p className="muted small">{BUTTON_COPY.privacy}</p>
      </Card>
      <Card className={styles.buttonCard}>
        <h2>{BUTTON_COPY.phoneTitle}</h2>
        <p className="muted">{BUTTON_COPY.phoneText}</p>
        <div>
          <Button icon={<Copy aria-hidden="true" />} onClick={copy}>
            {copied ? BUTTON_COPY.copied : BUTTON_COPY.copyCode}
          </Button>
        </div>
      </Card>
    </div>
  );
}
