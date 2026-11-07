import { useEffect, useId, useRef, type ReactNode } from "react";
import { X } from "lucide-react";
import { COMMON } from "../constants/copy";
import { IconButton } from "./Button";
import styles from "./Dialog.module.css";

interface DialogProps {
  open: boolean;
  title: string;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
  /** False keeps the dialog open on Escape and hides the close button. */
  dismissible?: boolean;
}

/** Native modal dialog: focus trapping and Escape come from the platform. */
export function Dialog({
  open,
  title,
  onClose,
  children,
  footer,
  dismissible = true,
}: DialogProps) {
  const reference = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  useEffect(() => {
    const dialog = reference.current;
    if (!dialog) {
      return;
    }
    if (open && !dialog.open) {
      dialog.showModal();
    }
    if (!open && dialog.open) {
      dialog.close();
    }
  }, [open]);
  return (
    <dialog
      ref={reference}
      className={styles.dialog}
      onClose={dismissible ? onClose : undefined}
      onCancel={(event) => {
        if (!dismissible) {
          event.preventDefault();
          return;
        }
        onClose();
      }}
      aria-labelledby={titleId}
    >
      {open && (
        <div className={styles.content}>
          <header className={styles.header}>
            <h2 id={titleId}>{title}</h2>
            {dismissible && (
              <IconButton label={COMMON.close} onClick={onClose}>
                <X aria-hidden="true" />
              </IconButton>
            )}
          </header>
          <div className={styles.body}>{children}</div>
          {footer && <footer className={styles.footer}>{footer}</footer>}
        </div>
      )}
    </dialog>
  );
}
