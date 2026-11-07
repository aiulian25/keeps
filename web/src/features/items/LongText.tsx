import { useLayoutEffect, useRef, useState } from "react";
import { DETAIL_COPY } from "./itemsCopy";
import styles from "./ItemDetail.module.css";

/**
 * Shows the first eight lines of long text (set by --long-text-lines in
 * ItemDetail.module.css); "Show more" appears only when text is cut off.
 */
export function LongText({ text }: { text: string }) {
  const reference = useRef<HTMLSpanElement>(null);
  const [expanded, setExpanded] = useState(false);
  const [overflowing, setOverflowing] = useState(false);

  useLayoutEffect(() => {
    const element = reference.current;
    if (!element || expanded) {
      return;
    }
    const measure = () =>
      setOverflowing(element.scrollHeight > element.clientHeight + 1);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, [text, expanded]);

  return (
    <span className={styles.longText}>
      <span
        ref={reference}
        className={
          expanded ? styles.preWrap : `${styles.preWrap} ${styles.clamped}`
        }
      >
        {text}
      </span>
      {(overflowing || expanded) && (
        <button
          type="button"
          className={styles.showMore}
          aria-expanded={expanded}
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? DETAIL_COPY.showLess : DETAIL_COPY.showMore}
        </button>
      )}
    </span>
  );
}
