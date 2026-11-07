import { useEffect, useState } from "react";
import { COPIED_NOTICE_MS } from "./constants";

/** Clipboard copy with a short-lived "Copied" flag for the button label. */
export function useCopied() {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) {
      return;
    }
    const timer = window.setTimeout(() => setCopied(false), COPIED_NOTICE_MS);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const copy = (text: string) => {
    navigator.clipboard.writeText(text).then(
      () => setCopied(true),
      () => setCopied(false),
    );
  };

  return { copied, copy };
}
